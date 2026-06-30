-- node3.cloud platform schema
-- Run once against a fresh Postgres instance before starting the platform service.

CREATE SCHEMA IF NOT EXISTS platform;

CREATE TABLE IF NOT EXISTS platform.accounts (
    id                    BIGSERIAL   PRIMARY KEY,
    telegram_id           BIGINT      NOT NULL UNIQUE,
    telegram_username     TEXT,
    did                   TEXT,
    public_key_hex        TEXT,        -- 65-byte uncompressed secp256k1 pubkey, hex-encoded (non-custodial DIDs only)
    node_id               INT         NOT NULL DEFAULT 0,
    tier                  TEXT        NOT NULL DEFAULT 'free' CHECK (tier IN ('free', 'paid')),
    lemon_subscription_id TEXT,
    next_billing_date     TIMESTAMPTZ,
    cancel_at             TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.sessions (
    id         BIGSERIAL   PRIMARY KEY,
    account_id BIGINT      NOT NULL REFERENCES platform.accounts(id) ON DELETE CASCADE,
    token      TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_token   ON platform.sessions (token);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON platform.sessions (expires_at);

CREATE TABLE IF NOT EXISTS platform.api_keys (
    id           BIGSERIAL   PRIMARY KEY,
    account_id   BIGINT      NOT NULL REFERENCES platform.accounts(id),
    key_hash     TEXT        NOT NULL UNIQUE,
    label        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at   TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ
);

-- Account-level monthly usage (aggregated; used for quota checks)
CREATE TABLE IF NOT EXISTS platform.account_usage (
    account_id    BIGINT NOT NULL REFERENCES platform.accounts(id) ON DELETE CASCADE,
    month         TEXT   NOT NULL, -- YYYY-MM
    request_count INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, month)
);

CREATE TABLE IF NOT EXISTS platform.webhook_subscriptions (
    id           BIGSERIAL   PRIMARY KEY,
    account_id   BIGINT      NOT NULL REFERENCES platform.accounts(id),
    -- api_key_id is nullable: subscriptions are account-scoped, not key-scoped
    api_key_id   BIGINT      REFERENCES platform.api_keys(id),
    event_type   TEXT        NOT NULL,
    filter_value TEXT        NOT NULL,
    callback_url TEXT        NOT NULL,
    secret       TEXT        NOT NULL,
    active       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS platform.webhook_deliveries (
    id              BIGSERIAL   PRIMARY KEY,
    subscription_id BIGINT      NOT NULL REFERENCES platform.webhook_subscriptions(id),
    transaction_id  TEXT        NOT NULL,
    attempted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status          TEXT        NOT NULL CHECK (status IN ('success', 'failed', 'pending')),
    response_code   INT
);

-- Cursor for the webhook worker. fullnode_transactions.id is TEXT (tx hash),
-- so pagination uses created_at rather than a sequence integer.
CREATE TABLE IF NOT EXISTS platform.webhook_cursor (
    id        INT         PRIMARY KEY DEFAULT 1,
    last_seen TIMESTAMPTZ NOT NULL DEFAULT 'epoch'
);
INSERT INTO platform.webhook_cursor (id, last_seen)
VALUES (1, 'epoch') ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS platform.hosted_contracts (
    id                 BIGSERIAL   PRIMARY KEY,
    account_id         BIGINT      NOT NULL REFERENCES platform.accounts(id),
    contract_id        TEXT        NOT NULL UNIQUE,
    wasm_artifact_hash TEXT,
    initial_state      JSONB       NOT NULL DEFAULT '{}',
    current_state      JSONB       NOT NULL DEFAULT '{}',
    deployed_at        TIMESTAMPTZ,
    execution_count    INT         NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS platform.contract_executions (
    id            BIGSERIAL   PRIMARY KEY,
    contract_id   TEXT        NOT NULL REFERENCES platform.hosted_contracts(contract_id),
    executed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    initiator_did TEXT,
    input         JSONB,
    output        JSONB,
    state_before  JSONB,
    state_after   JSONB,
    success       BOOLEAN     NOT NULL,
    error         TEXT
);

-- Pending signature contexts: created when a non-custodial DID deploy returns
-- "Signature needed" from the node. HandleTxSign pops this after consensus.
CREATE TABLE IF NOT EXISTS platform.pending_sign_contexts (
    sign_id    TEXT        PRIMARY KEY,  -- reqID from node "Signature needed" response
    action     TEXT        NOT NULL,     -- 'deploy' is the only action for now
    ref_id     TEXT        NOT NULL,     -- contractID when action='deploy'
    account_id BIGINT      NOT NULL REFERENCES platform.accounts(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Dedicated node pool. Admin pre-spins rubixgoplatform processes and inserts rows here.
-- id is the node_id (nodeutil.URL uses 20000+node_id as port); port is informational.
-- Example: INSERT INTO platform.nodes (id, port) VALUES (1, 20001), (2, 20002);
CREATE TABLE IF NOT EXISTS platform.nodes (
    id     INT  PRIMARY KEY,  -- node index: nodeutil.URL(id) = http://127.0.0.1:20000+id
    port   INT  NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'assigned'))
);

-- Index for the webhook worker: fast lookup by created_at on fullnode_transactions.
-- Run this separately against the rubix node database (not the platform DB):
--
-- CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_fullnode_txn_created_at
--   ON fullnode_transactions (created_at);
