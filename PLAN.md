# Rubix Node Provider Platform — Implementation Plan

> Status: **Draft — in review**
> Last updated: 2026-06-29
> Domain: **node3.cloud**
> This document is a living plan. Sections marked [TBD] are open for discussion before implementation begins.

---

## 1. Product Vision

**node3.cloud** is Rubix network infrastructure for developers — a managed node platform that removes the need to run your own node while giving you three capabilities that don't exist anywhere else in the Rubix ecosystem:

1. **Webhook subscriptions** — be notified in real time when anything happens on the network that your app cares about, instead of polling
2. **Non-custodial signing** — your private key stays in your browser/device; the hosted node participates in the network on your behalf but can never sign without you
3. **Smart contract hosting** — deploy your WASM contract and get a managed execution environment with a hosted dapp callback server, state storage, and execution logs; no server required

The raw API (balance queries, transfers, history) is baseline infrastructure — necessary but not the pitch. The three features above are what make this worth paying for. No other service in the Rubix ecosystem offers any of them.

Hosted at **node3.cloud**. Visual design: ChatGPT-style — clean sidebar, dark-neutral, no clutter. Every screen has one job.

**Core principle:** Every API call requires an API key. No unauthenticated surface.

---

## 2. User Tiers

| Tier      | Node model                                           | Requests/month | Price     |
|-----------|------------------------------------------------------|----------------|-----------|
| Free      | Shared node (DID on a node we manage)                | 10,000         | $0        |
| Pro       | Dedicated node (own IPFS peer ID, own P2P identity)  | 500,000        | $30/month |
| Unlimited | Dedicated node (own IPFS peer ID, own P2P identity)  | No limit       | $100/month|

**Key distinction — shared vs dedicated node:**
- A Rubix node can host hundreds of DIDs. Free tier users get a DID created on one of the platform's shared nodes — no node is provisioned per user. The node is an infrastructure detail they never see.
- Paid tier users get their own node process: a dedicated IPFS peer ID, dedicated P2P identity on the network, and resource isolation. This is what node providers charge for. It's a real differentiator — not just more requests.
- In both cases, the user gets one DID, one API key, all three platform features (webhooks, non-custodial signing, smart contract hosting).
- [TBD] What are the paid tier request limits and pricing per dedicated node?
- [TBD] Should requests that trigger consensus (transfers, deploys) cost more "credits" than read-only calls?

---

## 3. System Architecture

Everything runs on a **single VM**. nginx is the only public-facing process. All inter-service communication is over localhost — no TLS internally, no network hops.

The Platform Backend and API Gateway are **merged into one Go service** (`platform`). On the same VM they share the Postgres connection pool, the in-process key cache, and quota counters.

### 3.1 Read vs Write — the real split (verified against codebase)

Deep investigation of the codebase produced these concrete facts that shape the architecture:

**Every read endpoint on `rubixgoplatform` queries its own local Postgres only.**
`GET /rubix/v1/dids/{did}/balances` → `SELECT ... FROM tokens WHERE did=$1`
`GET /rubix/v1/tx/{did}/{token_type}` → `SELECT ... FROM transactions WHERE info->>'initiator'=$1`
Neither touches IPFS, peers, or any network-wide index.

**A regular node only knows what it participated in.**
After a transfer completes, only the initiator and receiver nodes write to `transactions` and `tokens`. Quorum nodes write pledge/unpledge entries only. No other nodes have any record.

**The full-node (`-fullnode` flag) indexes the entire network — but its data is not exposed via any REST endpoint.**
Full-node subscribes to the `rubix_txn` pubsub topic (verified: `constants/events.go`, singular not plural) and writes all observed transactions into a separate `fullnode_rbt / fullnode_transactions / fullnode_tokenchain` table family in the same Postgres instance. However, no HTTP route in `rubixgoplatform` queries these tables. The `-deexp` flag that was meant to expose them is dead code on `release-v1`.

**What `rubix_txn` covers (confirmed):** RBT transfers, FT minting and transfers, NFT mints and transfers, smart contract deploys and executes. **Not covered:** part-token genesis operations — `core/parts/parts.go` explicitly skips `PublishTransaction` for part-token genesis.

**Historical coverage is forward-only from the moment the full-node starts.**
There is no bulk backfill, no pubsub replay, no "give me all tokens on the network" API anywhere in the protocol. Tokens that never transact again after the full-node starts are invisible. The one partial relief: when a previously-unknown token *does* transact after the full-node starts, it retroactively fetches that token's complete prior chain from the initiator peer via `SyncTransactionChainsFromPeer`.

### 3.2 How reads are served — direct Postgres, not HTTP proxy

Because the full-node's `fullnode_*` tables live in the **same Postgres instance** as everything else on the VM, the Platform Go service reads them directly — no HTTP round-trip through the node.

| Request type | How it's served | Postgres tables |
|---|---|---|
| GET balance, tx history, tx by ID | Platform Go service queries Postgres directly | `fullnode_rbt`, `fullnode_transactions`, `fullnode_tokenchain` |
| POST transfer, mint, deploy, sign | Platform Go service proxies to user's assigned node (localhost:2000N) | Node writes to its local `tokens`, `transactions` tables; full-node picks it up via pubsub |

This is cleaner than proxying through the node's HTTP layer: no extra serialization, no dependency on node-side REST endpoints being correct, and the Platform service controls the query shape.

### 3.3 Architecture diagram

```
                         Internet
                             │
                    HTTPS node3.cloud
                             │
                    ┌────────▼────────┐
                    │      nginx       │  TLS termination (Let's Encrypt)
                    │   public :443    │  Only process exposed to internet
                    └──┬──────────┬───┘
                       │          │
              /        │          │  /api/*  and  /rubix/v1/*
              ▼        │          ▼
  ┌───────────────┐    │  ┌────────────────────────────────────────┐
  │   SvelteKit   │    │  │       Platform Go Service               │
  │  localhost    │    │  │        localhost:8080                    │
  │  :3000        │    │  │                                         │
  │  Web Portal   │    │  │  /api/*  → accounts, keys, billing      │
  │  (SSR)        │    │  │                                         │
  └───────────────┘    │  │  GET /rubix/v1/* → direct Postgres read │
                       │  │    queries fullnode_rbt,                │
                       │  │    fullnode_transactions, etc.           │
                       │  │                                         │
                       │  │  POST /rubix/v1/* → proxy to node       │
                       │  │    routes to user's assigned node        │
                       │  └──────────────────┬──────────────────────┘
                       │                     │  POST only, localhost
                       │          ┌──────────┴──────────┐
                       │          │                     │
                       │   ┌──────▼──────┐   ┌──────────▼──────┐
                       │   │  Full-Node  │   │  User Node(s)   │
                       │   │  :20000     │   │  :20001, :20002 │
                       │   │  -fullnode  │   │  user DIDs live │
                       │   │  pubsub →   │   │  here; handle   │
                       │   │  writes to  │   │  consensus      │
                       │   │  fullnode_* │   └─────────────────┘
                       │   └─────────────┘
                       │
                       │  Shared Postgres — same instance, all nodes
                       │  ┌──────────────────────────────────────────┐
                       └─▶│  fullnode_rbt  fullnode_transactions      │
                          │  fullnode_tokenchain  fullnode_tokenchain_index│
                          │  accounts  api_keys  nodes  usage         │
                          └──────────────────────────────────────────┘
```

### 3.4 Known limitation: forward-only coverage

The `fullnode_*` tables are only populated from the moment the full-node starts subscribing to pubsub. Tokens and transactions that completed before that point are invisible unless they transact again later (at which point their full prior chain is retroactively fetched).

**Practical impact at launch:**
- Balance queries on DIDs with pre-existing token history will return zero or incomplete results until those tokens move
- Transaction history will be incomplete for the same DIDs
- Coverage grows naturally over time as tokens transact through the network

**Mitigation options (not in scope for launch):**
- Contact Rubix core team about adding a protocol-level historical sync endpoint to `rubixgoplatform` (an `/rubix/v1/internal/enumerate_tokens` or similar)
- Build a one-time import tool that reads from a centralized Rubix explorer (if one exists and exposes historical data) to seed `fullnode_*` tables
- Document the limitation clearly in API docs with a `data_since` field in responses

---

## 4. Components

### 4.1 Web Portal (Frontend)

**Purpose:** Account management and feature control only — purely a developer control panel. No blockchain UI.

**Visual design:** ChatGPT-style — dark-neutral background (`#0f0f0f` base), clean sans-serif (Inter or Geist), left sidebar navigation, generous whitespace, subtle borders. Nothing decorative. Every screen has one primary action.

**Auth:** Telegram Login Widget — one click, no passwords, no email, no verification flow.

**App routes:**

| Route | Purpose |
|---|---|
| `/` | Landing page (detailed below) |
| `/dashboard` | DID, usage meter, active keys, webhook delivery health at a glance |
| `/keys` | Create, label, copy-once, revoke API keys |
| `/webhooks` | Create subscriptions (event + filter DID/contract + callback URL), delivery history |
| `/contracts` | Upload WASM, deploy, view execution logs and state history |
| `/billing` | Current plan, upgrade to dedicated node (Lemon Squeezy), invoice history |
| `/docs` | Redirect to Swagger API reference |

Note: no `/nodes` route for free tier — node assignment is invisible. The `/billing` upgrade flow reveals "dedicated node" as the paid benefit.

**Stack:** SvelteKit — SSR, file-based routing, minimal JS bundle. `localhost:3000` behind nginx.

---

### 4.1.1 Landing Page Content Plan (`/`)

The landing page has one job: convert a developer who landed on node3.cloud into someone who clicks "Login with Telegram." Every section earns the next.

---

**Above the fold — Hero**

```
Headline:    Build on Rubix. Without the infrastructure.

Subline:     A managed node platform with webhooks, non-custodial signing,
             and hosted smart contracts. Free to start.

CTA:         [Login with Telegram]   (primary, full-width on mobile)
             [Read the docs →]       (secondary, text link)
```

Visual: dark background, the three feature names appear as faint glowing labels behind the headline — purely atmospheric, not interactive. No hero image, no illustration.

---

**Section 1 — The problem (2 sentences, no header)**

```
Running a Rubix node means managing IPFS, P2P networking, port forwarding,
and a database — before writing a single line of your app.

node3.cloud does all of that so you don't have to.
```

---

**Section 2 — Feature 1: Webhooks**

```
Header:    Know when it happens. Not when you check.

Body:      Subscribe to any event on the Rubix network — a transfer to your DID,
           a smart contract execution, a new token mint — and get an HTTP POST
           to your server the moment it settles.

           No polling. No missed events. Just your callback URL.

Code snippet (dark card):
  POST https://your-app.com/hooks/rubix
  {
    "event": "token.received",
    "data": {
      "to_did": "bafybmi...",
      "amount": 1.5,
      "transaction_id": "..."
    }
  }
```

---

**Section 3 — Feature 2: Non-Custodial Signing**

```
Header:    Your keys. Always.

Body:      Your private key is derived from your secret phrase in your browser
           and never leaves your device. node3.cloud manages your node on the
           network but cannot sign a single transaction without you.

           Most hosted node services hold your keys. We don't.

Visual:    Two-column diagram — left: "Your device (key lives here)" →
           right: "node3.cloud (node participates, never signs)"
           Clean minimal arrows, no icons.
```

---

**Section 4 — Feature 3: Smart Contract Hosting**

```
Header:    Deploy a contract. Not a server.

Body:      Rubix smart contracts need a running callback server to handle
           executions. node3.cloud hosts it for you — upload your WASM,
           and every execution runs on our infrastructure with full logs,
           state history, and webhook events on each run.

Code snippet (dark card):
  POST /api/contracts/deploy
  { "wasm": "<base64>", "initial_state": { "count": 0 } }

  → Contract live. Executions logged. No server needed.
```

---

**Section 5 — Pricing (simple, honest)**

```
Header:    Simple pricing.

Two cards side by side:

  ┌─────────────────┐    ┌─────────────────┐
  │   Free           │    │   Pro            │
  │   $0/month       │    │   $[TBD]/month   │
  │                  │    │                  │
  │  1 DID           │    │  Dedicated node  │
  │  Shared node     │    │  Own P2P identity│
  │  10k req/month   │    │  [TBD] req/month │
  │  3 webhooks      │    │  Unlimited       │
  │  1 contract      │    │  webhooks        │
  │                  │    │  Multiple        │
  │                  │    │  contracts       │
  │ [Get started →]  │    │ [Upgrade →]      │
  └─────────────────┘    └─────────────────┘
```

---

**Section 6 — CTA strip**

```
Start building on Rubix in 30 seconds.

[Login with Telegram]
```

Small subtext: "No credit card required. No email. Just Telegram."

---

**Footer**

```
node3.cloud    Docs    GitHub (if public)    Telegram (support channel)
```

Minimal. One line. No cookie banners unless legally required.

---

**Design notes for the landing page:**
- No animations except subtle fade-in on scroll (CSS only, no JS libraries)
- Code snippets use a monospace font with syntax highlighting (Shiki via SvelteKit)
- The Telegram Login Widget is the only third-party JS on the page
- Mobile-first: hero CTA full-width, feature sections stack vertically
- No stock photos, no illustrations, no icons — text and code carry the whole page

---

### 4.2 Platform Go Service (Backend + Gateway, merged)

**Purpose:** Single Go binary serving two concerns over one HTTP port (`localhost:8080`):

- `/api/*` — account management, key issuance, billing, node assignment (consumed by the SvelteKit portal)
- `/rubix/v1/*` — gateway: validate key → check quota → **read: query Postgres directly / write: proxy to assigned node**

Merging these is the right call on a single VM: they share the same Postgres connection pool, the same in-process key cache, and the same quota counters. Splitting them would add IPC complexity with no benefit.

**Responsibilities:**
- Telegram OAuth token verification (no password storage)
- API key generation and storage (hash-only, shown once)
- Key-to-node assignment mapping
- Monthly quota tracking and reset
- Lemon Squeezy webhook handling
- Reverse proxy to `localhost:2000N` for validated developer requests

**Data model (core tables):**

```
accounts
  id, telegram_id, telegram_username, created_at,
  tier (free|pro|unlimited), lemon_subscription_id

api_keys
  id, account_id, key_hash, label, node_id,
  created_at, revoked_at

nodes
  id, label, network (mainnet|testnet),
  localhost_port, status (active|maintenance)

usage
  id, api_key_id, month (YYYY-MM),
  request_count, last_updated_at
```

**Request handling — two distinct paths:**

**Read path (GET requests):**
1. nginx forwards `GET /rubix/v1/*` with `X-API-Key` to `localhost:8080`
2. Validate key + check quota (same as write path)
3. Platform service executes Postgres query directly against `fullnode_*` tables
4. Returns shaped JSON response — no HTTP hop to any Rubix node
5. Increment usage counter async

**Write path (POST requests):**
1. nginx forwards `POST /rubix/v1/*` with `X-API-Key` to `localhost:8080`
2. Validate key + check quota
3. Resolve caller's assigned node: `node_id` → `localhost:<port>`
4. Reverse proxy raw request to that node's HTTP API
5. Node executes consensus, writes to its local tables; full-node picks it up via pubsub
6. Return node response verbatim to caller
7. Increment usage counter async

**Key queries the platform service executes directly:**

| Endpoint | SQL (simplified) |
|---|---|
| GET /rubix/v1/dids/{did}/balances/rbt | `SELECT sum(token_value) FROM fullnode_rbt WHERE did=$1 AND token_status=0` |
| GET /rubix/v1/dids/{did}/balances/ft | `SELECT * FROM fullnode_ft WHERE did=$1` |
| GET /rubix/v1/dids/{did}/balances/nft | `SELECT * FROM fullnode_nft WHERE did=$1` |
| GET /rubix/v1/tx/{did}/{token_type} | `SELECT * FROM fullnode_transactions WHERE info->>'initiator'=$1 OR info->>'owner'=$1` |
| GET /rubix/v1/tx/{tx_id} | `SELECT * FROM fullnode_transactions WHERE id=$1` |
| GET /rubix/v1/nfts/{id}/chain | `SELECT * FROM fullnode_tokenchain WHERE token_id=$1 ORDER BY position` |
| GET /rubix/v1/smart_contracts/{id}/chain | `SELECT * FROM fullnode_tokenchain WHERE token_id=$1 ORDER BY position` |

**Postgres schema separation:**
Platform tables (`accounts`, `api_keys`, `nodes`, `usage`) live in the `platform` schema. Rubix node tables (`fullnode_rbt`, `fullnode_transactions`, etc.) live in the `public` schema as written by `rubixgoplatform`. The platform service connects to the same Postgres instance with read access to `public` and full access to `platform`.

**Known consistency behaviors (document in API responses):**

- **Eventual consistency after writes:** After a POST (transfer, mint) completes on the user's node, the full-node picks it up from pubsub asynchronously — typically within seconds but not instant. A GET balance immediately after a POST will reflect the pre-transaction state. Clients should account for this or poll.
- **New DID blind spot:** After `POST /rubix/v1/dids/create`, the DID is local to the user's node and gossiped via pubsub. A balance query for that DID returns zero from `fullnode_rbt` until the DID has an associated token transaction. This is expected, not a bug.
- **Forward-only coverage:** `fullnode_*` tables only contain transactions observed since the full-node started. Pre-existing history is absent until those tokens transact again (at which point full chain is retroactively fetched). API responses should include a `data_since` field.

**Stack:** Go — consistent with `rubixgoplatform`, single binary deployment, no runtime dependencies

---

### 4.4 Rubix Nodes (`rubixgoplatform`)

**Node index allocation:**
- `node_index=0` — full-node (`-fullnode` flag), populates `fullnode_*` tables. Also serves as the shared node for all free tier users — their DIDs are created here. `-fullnode` is independent of `node_index`; the same process handles both indexing and DID hosting.
- `node_index=1, 2, 3, ...` — dedicated nodes for paid tier users. One process per paid user. Each gets its own IPFS peer ID and P2P identity on the network.

**How DID assignment works:**
- Free tier signup: platform calls `POST /rubix/v1/dids/create` on node_index=0 → DID created in that node's filesystem → platform stores `account → did → node_id=0`
- Paid tier upgrade: platform spawns a new node process (next available `node_index`), creates the DID there → `account → did → node_id=N`
- Write requests are routed to `localhost:20000+node_id` — the node that owns the DID
- Read requests always go to Postgres directly (no node routing needed)

**Changes needed to the existing codebase:**

| Change | File | Detail |
|--------|------|--------|
| Full-node runs with `-fullnode` | operational | No code change — just launch `node_index=0` with the `-fullnode` flag |
| Single shared secret per node | `command/command.go` | Each node's `EnableAuth` API key is a platform-internal secret, never exposed to end users |
| Production TLS flag | `command/command.go:333` | Wire `Production`, `CertFile`, `KeyFile` from `config.toml` instead of hardcoded `"false"` |
| Node health endpoint | `server/node.go` | Simple `GET /rubix/v1/node/status` returning node state — used by platform service for liveness probing |

**What does NOT need changing (investigated and confirmed):**
- Read routes do not need to be opened or modified — the platform service bypasses the node's HTTP layer entirely for reads and queries `fullnode_*` Postgres tables directly
- No new read endpoints need to be added to the node

**Node network modes:**
- `mainnet` — production Rubix network
- `testnet` — for developers testing integrations
- [TBD] Should free tier get mainnet, testnet, or user's choice?

**Port layout per node (from `constants/ipfs.go`, offset by `node_index`):**

| Service         | Base Port | Full-Node (idx=0) | User Node 1 (idx=1) | User Node N |
|-----------------|-----------|-------------------|---------------------|-------------|
| Rubix HTTP API  | 20000     | 20000             | 20001               | 20000+N     |
| P2P Send        | 21000     | 21000             | 21001               | 21000+N     |
| P2P Recv        | 22000     | 22000             | 23000               | 22000+(1000×N) |
| IPFS API        | 5002      | 5002              | 5003                | 5002+N      |
| IPFS Swarm      | 4002      | 4002              | 4003                | 4002+N      |
| IPFS HTTP GW    | 8081      | 8081              | 8082                | 8081+N      |
| Postgres        | 5433      | 5433              | 5434                | 5433+N      |

**Important — Postgres per node:** Default is a separate Postgres instance per node (port `5433 + node_index`, base is 5433 not 5432). Confirmed in `core/config/config.go:149-153`: port defaults to `PostgresBasePort + NodeIndex` unless `db.port` is explicitly set in `config.toml`. To share one Postgres instance across all nodes, set the same `db.port` and use distinct `db.db_name` per node — this is supported via manual config override.

---

### 4.5 Billing (Lemon Squeezy)

Chosen over Stripe for minimal setup: Lemon Squeezy acts as the **merchant of record** — they handle VAT, GST, and sales tax globally. You create a subscription product in their dashboard, get a checkout URL, and handle a small number of webhooks. No PCI compliance surface, no tax registration in multiple countries.

**Setup steps (one-time):**
1. Create account at Lemon Squeezy
2. Create a "Paid Node Plan" subscription product — set price and billing interval
3. Note the `variant_id` — used to identify the plan in webhooks
4. Register webhook endpoint: `https://node3.cloud/api/lemon/webhook`
5. Store the webhook signing secret in env config

**Checkout flow:**
- User clicks "Upgrade" in the portal
- Platform service generates a Lemon Squeezy checkout URL with `custom_data: { account_id }` pre-filled
- User completes payment on Lemon Squeezy hosted page
- Lemon Squeezy calls our webhook

**Webhooks to handle (only 3):**

| Event | Action |
|---|---|
| `subscription_created` | Set `accounts.tier = paid`, store `lemon_subscription_id` |
| `subscription_cancelled` | Set `accounts.tier = free` at period end, remove extra nodes |
| `subscription_payment_failed` | Grace period of [TBD] days, then downgrade |

**No SDK needed** — verify the webhook signature with HMAC-SHA256 against the `X-Signature` header, parse the JSON body, done.

---

## 5. Core Features (the three differentiators)

### 5.1 Webhooks

**The problem it solves:**
Today if you want to know when a DID receives tokens or when a smart contract executes, you have to poll the API. There's no event system. For any real dApp this is a dealbreaker.

**How it works:**
The full-node subscribes to the `rubix_txn` pubsub topic (confirmed: `constants/events.go`) and writes all observed transactions to `fullnode_transactions`. The platform service's background worker polls this table and, for each new row, checks registered webhook subscriptions. Matches trigger an HTTP POST to the developer's callback URL.

**What developers can subscribe to:**

| Event | How detected in `fullnode_transactions.info` |
|---|---|
| `token.received` | `info->>'owner' = watchedDID` AND `info->'tokens'->'rbt'` or `'ft'` or `'nft'` is non-empty |
| `token.sent` | `info->>'initiator' = watchedDID` AND token arrays non-empty |
| `contract.deployed` | `info->'tokens'->'smartContract'` non-empty AND `previousTransactionID = ""` |
| `contract.executed` | `info->'tokens'->'smartContract'` non-empty AND `previousTransactionID != ""` |

**Removed from scope:** `did.registered` — DID events travel on a separate pubsub topic, not `rubix_txn`. Full-node does not index them. Not feasible with current protocol.

**Important limitation — SC execution output:** The `contract.executed` webhook signals that execution happened and provides `smart_contract_data` (the input). However the WASM execution result/output is NOT in `fullnode_transactions` — it travels on a separate per-contract pubsub topic only delivered to subscribed nodes. The webhook will confirm execution occurred and include the input, but not the output state. This is a protocol-level constraint.

**Subscription model:**
- Developer registers via `POST /api/webhooks` — specifies event type, filter value (DID or contract token ID), and callback URL
- Platform service stores subscriptions in `platform.webhook_subscriptions`
- Background worker polls `fullnode_transactions WHERE created_at > last_processed_cursor` — cursor is `created_at`, NOT `info->>'epoch'` (epoch is the initiator's clock; `created_at` is when the full-node committed the row, which is the correct ordering field)
- Retries with exponential backoff on failure (3 attempts), writes to `webhook_deliveries`
- Dashboard shows delivery history, success/failure per webhook

**Required DB index:** `CREATE INDEX ON fullnode_transactions ((info->>'owner'))` — the `fullnode_smart_contract` table has no `did` column, so SC event filtering must parse `info->>'owner'` via JSON expression. This index makes the worker's matching query fast at scale.

**New platform DB tables:**
```
webhook_subscriptions
  id, account_id, api_key_id, event_type, filter_value (DID or contract_id),
  callback_url, secret (for HMAC signature on payload), created_at, active

webhook_deliveries
  id, subscription_id, transaction_id, attempted_at, status, response_code
```

**Payload sent to developer's URL:**
```json
{
  "event": "token.received",
  "timestamp": 1719619200,
  "data": {
    "transaction_id": "...",
    "to_did": "bafybmi...",
    "from_did": "bafybmi...",
    "token_type": "rbt",
    "amount": 1.5
  }
}
```
Header: `X-Rubix-Signature: sha256=<HMAC of payload using subscription secret>` — same pattern as GitHub webhooks.

**Implementation notes:**
- rubixgoplatform does NOT use Postgres LISTEN/NOTIFY anywhere (confirmed by grep) — the platform service can use it freely on the same Postgres instance without conflict; polling is the simpler starting point
- Webhook fanout is async — does not block the API response
- Free tier: up to 3 active webhook subscriptions
- Paid tier: unlimited
- Part-token genesis transactions are NOT published to `rubix_txn` — cannot be caught by webhooks (protocol limitation, not a platform limitation)

---

### 5.2 Non-Custodial Signing

**The problem it solves:**
In the current plan (and in any naive hosted node), the user's DID private key lives on the server. The provider can sign transactions on the user's behalf without consent. For any serious use — wallets, dApps handling real value — this is unacceptable.

**How Rubix keys work (confirmed from codebase):**
LiteDID mode (the default, `did/lite.go`) derives a secp256k1 keypair from a BIP-39 mnemonic at path `m/44'/9999'/0'/0/0`. The mnemonic → private key derivation is deterministic and can run entirely in the browser (WebCrypto or a JS BIP-39 library). The public key is what gets registered as the DID (it's the IPFS CID of the public key file).

**The flow:**
1. User generates a mnemonic in-browser (or imports their own) — never sent to the server
2. Browser derives the secp256k1 keypair locally (uncompressed, 65 bytes)
3. Browser sends only the **public key (hex)** to the platform: `POST /api/did/register-pubkey`
4. Platform calls `POST /rubix/v1/dids/create` with `{"public_key":"<hex>"}` — this branches to `CreateDIDFromPubKey` in `did/did.go:284`, which writes only `pubKey.pem` to the DID directory. No private key file is ever created.
5. The DID is now live on the network. The server has `pubKey.pem` and the DID string. It has never seen the private key.

**Correction from codebase investigation:** The plan previously referenced `APIRequestDIDForPubKey` — this constant and handler **do not exist** in `release-v1`. The correct endpoint is `POST /rubix/v1/dids/create` with the `public_key` field populated. Confirmed in `core/did.go` `CreateDID` handler.

**For signing transactions — the actual protocol (two round-trips):**

When no private key file exists on the node, `did/lite.go:Sign()` falls back to `getSignature()` which returns an intermediate "Signature needed" response containing the hash to be signed. This is the built-in non-custodial signing protocol. The full flow:

**Round 1 — Initiate:**
```
POST /rubix/v1/tx  { initiator, owner, tokens: {...} }
← HTTP 200 { status: false, message: "Signature needed", result: { id: "<reqID>", hash: "<hex>" } }
```

**Client — sign in browser:**
```
signatureBytes = secp256k1.sign(hash, privateKey)  // runs in-browser, never touches server
```

**Round 2 — Submit signature:**
```
POST /rubix/v1/signature  { id: "<reqID>", signature: "<base64>" }
← HTTP 200 { status: true, message: "Transaction completed", result: { txID: "..." } }
```

The platform service must hold the `reqID` between the two calls — it is the correlation handle the node uses to resume the blocked transaction goroutine. The client waits for round 2's response as confirmation.

This same two-round-trip protocol applies to: `POST /rubix/v1/tx` (all transaction types), `POST /rubix/v1/dids/{did}/register`, and `POST /rubix/v1/signature/arbitrary`.

**What the user sees:**
- On signup: "Generate your secret phrase" → 12-word mnemonic shown once, user saves it
- On every session: "Enter your password to unlock" → key derived in-browser
- Server never receives the mnemonic or private key

**Implementation notes:**
- A small JS library handles BIP-39 mnemonic generation and secp256k1 signing in-browser
- The platform service exposes a thin relay API: `POST /api/tx/initiate` → calls node, returns hash + reqID to client; `POST /api/tx/sign` → posts signature to node, returns final result
- Free tier and paid tier both get non-custodial option; it's a trust feature, not a paid upsell

---

### 5.3 Smart Contract Hosting

**The problem it solves:**
Rubix smart contracts are WASM programs. When a contract is executed, the node sends an HTTP callback to a "dapp host" — a server the developer has to run themselves. This server receives the execution context, runs business logic, and returns the new state. Most developers don't want to run a server just to use smart contracts.

**What the platform provides:**
- A managed dapp host running inside the Platform Go service
- Developer uploads their `.wasm` binary; platform handles generate → register callback → deploy on the node
- Platform registers itself as the callback URL: `https://node3.cloud/api/contracts/{contract_id}/callback`
- On each execution, the node calls the platform's callback endpoint; the platform runs the WASM, manages state in Postgres, returns result
- Developer sees a dashboard of every execution: timestamp, input, output, state diff, success/failure

**Lifecycle from developer perspective:**
1. `POST /api/contracts/deploy` — upload `.wasm` + `.rs` files, optionally initial state JSON
2. Platform calls `POST /rubix/v1/smart_contracts/generate` (multipart: `did`, `binaryCodePath`, `rawCodePath`) → node pins to IPFS, returns `contract_token_id` (Qm... CID)
3. Platform stores WASM binary itself (downloaded from IPFS using `artifactHash` from the generate response, via `GET /rubix/v1/smart_contracts/fetch`)
4. Platform calls `POST /rubix/v1/smart_contracts/register_callback` with `{SmartContractToken, CallBackURL}` — can be done before deploy
5. Platform calls `POST /rubix/v1/tx` with `tokens.smartContract[{smartContractId, value, data}]` — node detects DEPLOY because token doesn't exist yet in DB; runs consensus, records genesis block
6. Any execution: caller calls `POST /rubix/v1/tx` (or the platform's own `/api/contracts/{id}/execute`) → node detects EXECUTE, runs consensus, publishes pubsub → subscribing node fires platform's callback
7. Platform callback receives POST, loads WASM, runs it, saves state and execution log
8. Developer queries `GET /api/contracts/{contract_id}/executions` for logs

**Correction from codebase investigation:** The plan previously referenced `POST /rubix/v1/smart_contracts/deploy` — this endpoint does not exist on release-v1. Both deploy and execute go through `POST /rubix/v1/tx`. The node auto-distinguishes: if the `smartContractId` token doesn't exist in its DB → deploy; if it does → execute.

**Callback protocol (confirmed from `core/smart_contract.go:ContractCallBack`):**

The callback fires on the **subscribing node** (not the executing node). The deploying node auto-subscribes; no separate subscribe call needed if platform is co-located with the deploying node.

Node sends to platform callback URL:
```json
POST https://node3.cloud/api/contracts/{contract_id}/callback
{
  "smart_contract_hash": "<Qm... contract token ID>",
  "port":                20000,
  "smart_contract_data": "<opaque string from execute request>",
  "initiator_did":       "<DID of transaction initiator>"
}
```

Platform must respond with HTTP 200:
```json
{ "message": "<any string>" }
```
Non-200 or missing `message` field: node logs an error, no retry.

**WASM runtime:** The node does NOT run WASM — it treats the `.wasm` file as an opaque blob on IPFS. The platform service is entirely responsible for the WASM execution engine. **Wasmtime is NOT a dependency of rubixgoplatform** (confirmed: `go.mod` has no WASM runtime). The platform must add `wasmer-go` or equivalent as its own dependency.

**New platform DB tables:**
```
hosted_contracts
  id, account_id, contract_id (Rubix CID), wasm_blob, wasm_artifact_hash,
  initial_state (JSONB), current_state (JSONB), deployed_at, execution_count

contract_executions
  id, contract_id, executed_at, initiator_did,
  input (JSONB), output (JSONB),
  state_before (JSONB), state_after (JSONB), success, error
```

**Implementation notes:**
- Platform must bring its own WASM runtime as a Go module dependency
- Free tier: 1 hosted contract
- Paid tier: multiple contracts, higher execution limits [TBD]
- Contract state is append-only in `contract_executions` — full audit trail
- `webhook fires (if subscribed)` still applies — after each execution, the webhook worker will pick it up from `fullnode_transactions` (the token chain event) and fire any matching `contract.executed` subscriptions

---

## 6. API Key Design

**Format:** `rbx_<env>_<32-byte-random-hex>`
- `rbx_live_...` for mainnet keys
- `rbx_test_...` for testnet keys

**Storage:** Store only the **hash** (SHA-256) of the key in the DB. Show the full key once at creation — never again. Same model as GitHub PATs.

**Key capabilities:**
- Keys are mapped to an account (and by extension to the account's DID and its node)
- Keys can be labeled (e.g. "my dapp prod", "testing")
- Free tier: 1 key
- Paid tier: multiple keys per account (per-app isolation without provisioning separate nodes)

---

## 7. Quota Enforcement

**Counting strategy:**
- Increment per HTTP request that reaches the node, regardless of success/failure
- Count resets on the 1st of each month (or rolling 30-day window — [TBD])
- Counter stored in platform backend DB, optionally cached in Redis for hot path

**What counts toward quota:**
- All API calls including reads

**[TBD] Weighted counting:**
- Read calls (GET balance, GET txn): 1 credit
- Write/consensus calls (POST transfer, POST deploy): 5–10 credits
- Rationale: consensus calls consume quorum peer resources, not just local node

**At quota limit:**
- Return `HTTP 429` with `Retry-After` header pointing to next reset date
- Telegram bot message at 80% and 100% of quota (no email — auth is Telegram-only; [TBD] in-portal alert as fallback if user has no bot interaction)

---

## 8. Node Assignment & DID Provisioning

**Free tier — DID on shared node (zero provisioning overhead):**
1. User logs in via Telegram for the first time
2. Platform calls `POST /rubix/v1/dids/create` on node_index=0 (the shared node / full-node)
3. DID is created, stored in `platform.accounts.did`; `node_id=0` recorded in `platform.accounts`
4. Platform generates API key, shows it once
5. Done — user never sees or selects a node

**Paid tier — dedicated node provisioned on upgrade:**
1. User clicks "Upgrade" in billing
2. Lemon Squeezy webhook fires `subscription_created`
3. Platform allocates next available `node_index` (or launches a new rubixgoplatform process if none pre-spun)
4. Migrates user's DID to the new node OR creates a fresh DID on it [TBD — migrate or fresh DID?]
5. Updates `platform.accounts.node_id` to the new node
6. User's writes now route to the dedicated node

**Platform DB change to `accounts` table:**
```
accounts
  id, telegram_id, telegram_username, created_at,
  tier (free|pro|unlimited), lemon_subscription_id,
  did,          ← user's Rubix DID string
  node_id       ← FK to nodes table; determines write routing
```

**Node pool strategy:**
- Launch with 1 shared node (node_index=0, doubles as full-node) and 1–2 pre-spun dedicated nodes ready for paid upgrades
- Pre-spun is simpler than on-demand: nodes take time to start (IPFS peer discovery, DB init), pre-warming avoids latency on the upgrade path
- Each dedicated node needs: its own `config.toml`, its own `db_name` in shared Postgres, its own IPFS data dir

**VM capacity:**
- node_index=0 (shared + full-node): ~600MB RAM (one IPFS daemon + rubixgoplatform)
- Each additional dedicated node: ~600MB RAM
- All other services (platform, SvelteKit, Postgres, nginx): ~1.4GB
- **4 vCPU / 8GB RAM handles ~10 dedicated nodes comfortably** — enough for launch
- Scale to 8 vCPU / 16GB when dedicated node count approaches 10

---

## 9. What Exists vs What Needs Building

### Already in `rubixgoplatform` (release-v1)
- Versioned REST routes `/rubix/v1/*`
- `EnableAuth` + `X-API-Key` header support
- PostgreSQL backend
- Full-node mode (`-fullnode` flag) for indexing all transactions
- `config.toml`-based configuration
- Swagger docs

### Needs building in `rubixgoplatform`
- Run the designated full-node with `-fullnode` flag — no code change, just operational
- Wire `Production`/TLS config properly (currently hardcoded `"false"` at `command/command.go:333`)
- Node health/status endpoint for gateway liveness probing
- **No new read endpoints needed** — platform service reads `fullnode_*` tables directly from Postgres
- **Security fix — unauthenticated routes bypass `EnableAuth`:** Confirmed in `server/server.go` that these 5 routes are registered without `s.AuthHandle()` wrapping and are accessible with no API key even when `EnableAuth=true`:
  - `POST /rubix/v1/dids/create` — unauthenticated DID creation (significant: anyone can create DIDs on the node)
  - `GET /rubix/v1/tx` — unauthenticated transaction list
  - `GET /rubix/v1/tx/{tx_id}` — unauthenticated tx by ID
  - `GET /rubix/v1/tx/{did}/{token_type}` — unauthenticated txs by DID
  - `GET /rubix/v1/fts` — unauthenticated FT list
  
  These must be wrapped with `s.AuthHandle()` in `server/server.go` before exposing any node to the internet. This is a required codebase change for launch.

### What was investigated and ruled out
- Routing reads through the node's HTTP API — not viable; all read endpoints query local-only tables, not `fullnode_*`
- Full-node REST API for balance queries — `fullnode_*` tables exist but no endpoint exposes them; `-deexp` flag is dead code
- Historical backfill on fresh full-node start — no mechanism exists in the protocol; forward-only coverage is the reality at launch
- Network-wide token enumeration — no such API exists; `sync_transaction_chain` requires knowing token IDs upfront; `ipfs_provider_store` is local-only

### New services to build (outside this repo)
- Platform Go service — accounts, keys, quotas, billing, and gateway (read: direct Postgres, write: proxy to node) — one binary
- SvelteKit web portal — account management UI only

---

## 10. Open Questions (resolve before implementation)

**Resolved:**
- ~~Platform backend stack~~ → Go
- ~~API Gateway implementation~~ → merged into Platform Go service, reverse proxy over localhost
- ~~Frontend stack~~ → SvelteKit
- ~~Deployment topology~~ → single VM, nginx edge

**Resolved by codebase investigation:**
- ~~Read routing~~ → Platform Go service queries `fullnode_*` tables directly from Postgres; no HTTP proxy for reads
- ~~Full-node as read endpoint~~ → not viable; endpoints query local tables only; direct DB access is the solution
- ~~Historical backfill~~ → not possible with current protocol; forward-only coverage is the launch reality; document clearly in API responses
- ~~Full-node pubsub topic~~ → `rubix_txn` (singular); covers RBT/FT/NFT/SC; part-token genesis NOT covered
- ~~Shared Postgres~~ → requires explicit `db.port` override in `config.toml` to share one instance; confirmed supported
- ~~`APIRequestDIDForPubKey` endpoint~~ → does not exist; correct path is `POST /rubix/v1/dids/create` with `public_key` field
- ~~Non-custodial signing protocol~~ → two round-trips: initiate → "Signature needed" + hash → POST signature; node never needs private key
- ~~Wasmtime in rubixgoplatform~~ → not present in `go.mod`; platform must add its own WASM runtime
- ~~SC deploy/execute endpoints~~ → both use `POST /rubix/v1/tx`; node auto-distinguishes by token existence in DB
- ~~Unauthenticated routes with EnableAuth~~ → confirmed 5 routes bypass AuthHandle; must be patched before launch
- ~~Node per user~~ → wrong model; one DID per user on a shared node (free) or dedicated node (paid); hundreds of DIDs per node is supported and correct

**Still open:**
1. ~~**Paid tier limits**~~ → Resolved: Free=10k, Pro=$30/500k, Unlimited=$100/no limit
2. **Consensus call weighting** — Do write ops (transfer, deploy) cost more credits than reads?
3. **Free tier network** — Mainnet, testnet, or user's choice?
4. **Overage policy** — Hard block at 10k or charge per additional 1000 requests?
5. **DID migration on upgrade** — When a free user upgrades to paid, do they get a fresh DID on the dedicated node, or does the platform migrate their existing DID (move filesystem dirs + update DB)? Fresh DID is simpler; migration preserves identity but requires careful node coordination.
6. **Rolling vs calendar quota reset** — 30-day rolling window or 1st of month?
7. **Multi-key per account** — Paid users can have multiple keys (confirmed above). Can free tier also have multiple keys for per-app isolation, or strictly 1?
8. **Testnet access** — Separate testnet key at signup, or same key routes to testnet via flag/subdomain?
9. **Telegram bot** — Quota alerts and key management via chat bot alongside the web portal?
10. **Historical coverage transparency** — API responses include `data_since` timestamp?

---

## 11. Implementation Phases (tentative, order subject to change)

### Phase 1 — VM foundation
- nginx: TLS (Let's Encrypt), routing for `/`, `/api/*`, `/rubix/v1/*`
- PostgreSQL: shared instance on port 5433; `platform` schema (`accounts`, `api_keys`, `nodes`, `usage`, `webhook_subscriptions`, `webhook_deliveries`, `hosted_contracts`, `contract_executions`)
- Patch `rubixgoplatform` `server/server.go`: wrap the 5 unauthenticated routes with `s.AuthHandle()`
- Wire `Production`/TLS config in `rubixgoplatform` (`command/command.go:333`)
- Launch node_index=0 (`-fullnode`, `db_name=rubix_node0`) — shared node for free tier DIDs + full-node indexer; begins populating `fullnode_*` tables
- Pre-spin 1–2 dedicated nodes (node_index=1+, `db_name=rubix_node1` etc.) ready for paid tier
- Add node health endpoint to `rubixgoplatform`

### Phase 2 — Platform Go service core (no billing, no features yet)
- Telegram OAuth verification + account creation
- On first login: call `POST /rubix/v1/dids/create` on node_index=0 → store DID + `node_id=0` in `accounts`
- API key generation, hash-only storage, retrieval
- Read path: direct Postgres against `fullnode_*` tables
- Write path: key validation + quota check + reverse proxy to `localhost:20000+account.node_id`
- Testable: login → DID auto-created → get key → query balance → initiate transfer

### Phase 3 — Webhooks
- `webhook_subscriptions` CRUD API (`POST/GET/DELETE /api/webhooks`)
- Background worker: tail `fullnode_transactions`, match subscriptions, fire callbacks
- Retry with exponential backoff, write to `webhook_deliveries`
- Dashboard: `/webhooks` page in SvelteKit showing subscriptions and delivery history

### Phase 4 — Non-custodial signing
- JS library: BIP-39 mnemonic generation + secp256k1 derivation in-browser
- DID creation flow: browser sends public key hex → `POST /rubix/v1/dids/create` with `{"public_key":"<hex>"}` → no private key on server
- Platform relay API: `POST /api/tx/initiate` (calls node, returns `{id, hash}` to client) + `POST /api/tx/sign` (posts client signature to node, returns final result)
- `/dashboard` shows DID + key mode (custodial vs non-custodial)

### Phase 5 — Smart contract hosting
- Add wasmer-go (or equivalent) as platform Go module dependency
- `POST /api/contracts/deploy` — upload `.wasm` + `.rs`, platform calls: generate → register_callback → `POST /rubix/v1/tx` (deploy)
- Expose `POST /api/contracts/{id}/callback` — receives node's callback, loads WASM, runs it, saves state + execution log
- State stored in `hosted_contracts`, executions logged to `contract_executions`
- `/contracts` page: deployment UI, execution log, state diff viewer
- Webhook `contract.executed` fires via Phase 3 worker on `fullnode_transactions`

### Phase 6 — SvelteKit web portal (landing + dashboard)
- Landing page (all 6 sections per 4.1.1 content plan)
- Full dashboard: DID display, usage meter, webhook health, contract count
- `/billing` stub (Lemon Squeezy not wired yet) — no `/nodes` route

### Phase 7 — Billing
- Lemon Squeezy Checkout integration
- 3 webhook handlers: `subscription_created` (provision dedicated node, create/migrate DID), `subscription_cancelled` (downgrade to shared node), `payment_failed` (grace period then downgrade)
- Paid tier unlocks: dedicated node, unlimited webhooks, multiple contracts
- Overage enforcement

### Phase 8 — Hardening
- In-process key cache (warm from Postgres on startup, invalidate on revoke)
- Per-key burst rate limiting at nginx level
- Telegram bot: quota alerts at 80% and 100%
- Internal node health monitoring + alerting
