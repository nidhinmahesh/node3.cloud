// Package db manages the Postgres connection pool and provides typed query
// helpers for both the platform schema and the fullnode_* tables.
package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"node3.cloud/platform/migrations"
)

// DB wraps a pgx connection pool.
type DB struct {
	pool      *pgxpool.Pool
	rubixPool *pgxpool.Pool // pool for the rubix node DB (may equal pool in dev)
}

// New opens a connection pool to the given DSN and auto-applies the platform
// schema (all statements are idempotent — CREATE TABLE IF NOT EXISTS etc.).
func New(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	if _, err := pool.Exec(ctx, migrations.PlatformSchema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &DB{pool: pool, rubixPool: pool}, nil
}

// NewWithRubixPool opens two pools: one for the platform DB and one for the
// rubix node DB (where fullnode_transactions lives). Pass the same DSN for
// both in single-instance dev deployments.
func NewWithRubixPool(ctx context.Context, platformDSN, rubixDSN string) (*DB, error) {
	d, err := New(ctx, platformDSN)
	if err != nil {
		return nil, err
	}
	if rubixDSN == platformDSN || rubixDSN == "" {
		return d, nil
	}
	rubixPool, err := pgxpool.New(ctx, rubixDSN)
	if err != nil {
		return nil, fmt.Errorf("rubix pgxpool.New: %w", err)
	}
	if err := rubixPool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("rubix db ping: %w", err)
	}
	d.rubixPool = rubixPool
	return d, nil
}

// Close releases all pool connections.
func (d *DB) Close() {
	d.pool.Close()
	if d.rubixPool != d.pool {
		d.rubixPool.Close()
	}
}

// ── Model types ──────────────────────────────────────────────────────────────

type Account struct {
	ID                  int64
	TelegramID          int64
	TelegramUsername    string
	DID                 string
	PublicKeyHex        string // 65-byte secp256k1 pubkey hex (non-custodial only)
	NodeID              int
	Tier                string
	LemonSubscriptionID string
	NextBillingDate     *time.Time
	CancelAt            *time.Time
	CreatedAt           time.Time
}

type PendingSignContext struct {
	SignID    string
	Action    string
	RefID     string
	AccountID int64
	CreatedAt time.Time
}

type Session struct {
	ID        int64
	AccountID int64
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type APIKey struct {
	ID         int64
	AccountID  int64
	KeyHash    string
	Label      string
	CreatedAt  time.Time
	RevokedAt  *time.Time
	LastUsedAt *time.Time
}

type Usage struct {
	AccountID    int64
	Month        string
	RequestCount int
}

type WebhookSubscription struct {
	ID          int64
	AccountID   int64
	EventType   string
	FilterValue string
	CallbackURL string
	Secret      string
	Active      bool
	CreatedAt   time.Time
}

type WebhookDelivery struct {
	ID             int64
	SubscriptionID int64
	TransactionID  string
	AttemptedAt    time.Time
	Status         string
	ResponseCode   *int
}

type HostedContract struct {
	ID             int64
	AccountID      int64
	ContractID     string
	WASMArtifactHash string
	InitialState   json.RawMessage
	CurrentState   json.RawMessage
	DeployedAt     *time.Time
	ExecutionCount int
}

type ContractExecution struct {
	ID           int64
	ContractID   string
	ExecutedAt   time.Time
	InitiatorDID string
	Input        json.RawMessage
	Output       json.RawMessage
	StateBefore  json.RawMessage
	StateAfter   json.RawMessage
	Success      bool
	Error        string
}

// ── Account queries ──────────────────────────────────────────────────────────

func (d *DB) UpsertAccount(ctx context.Context, telegramID int64, username string) (*Account, error) {
	row := d.pool.QueryRow(ctx, `
		INSERT INTO platform.accounts (telegram_id, telegram_username)
		VALUES ($1, $2)
		ON CONFLICT (telegram_id) DO UPDATE
		  SET telegram_username = EXCLUDED.telegram_username
		RETURNING id, telegram_id, telegram_username, COALESCE(did,''), COALESCE(public_key_hex,''),
		          node_id, tier, COALESCE(lemon_subscription_id,''), next_billing_date, cancel_at, created_at
	`, telegramID, username)
	return scanAccount(row)
}

func (d *DB) GetAccountByID(ctx context.Context, id int64) (*Account, error) {
	row := d.pool.QueryRow(ctx, `
		SELECT id, telegram_id, telegram_username, COALESCE(did,''), COALESCE(public_key_hex,''),
		       node_id, tier, COALESCE(lemon_subscription_id,''), next_billing_date, cancel_at, created_at
		FROM platform.accounts WHERE id = $1
	`, id)
	return scanAccount(row)
}

func (d *DB) GetAccountByLemonID(ctx context.Context, subID string) (*Account, error) {
	row := d.pool.QueryRow(ctx, `
		SELECT id, telegram_id, telegram_username, COALESCE(did,''), COALESCE(public_key_hex,''),
		       node_id, tier, COALESCE(lemon_subscription_id,''), next_billing_date, cancel_at, created_at
		FROM platform.accounts WHERE lemon_subscription_id = $1
	`, subID)
	return scanAccount(row)
}

func (d *DB) SetAccountDID(ctx context.Context, accountID int64, did string) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE platform.accounts SET did = $1 WHERE id = $2`,
		did, accountID)
	return err
}

func (d *DB) SetAccountPublicKey(ctx context.Context, accountID int64, publicKeyHex string) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE platform.accounts SET public_key_hex = $1 WHERE id = $2`,
		publicKeyHex, accountID)
	return err
}

func (d *DB) SetAccountNodeID(ctx context.Context, accountID int64, nodeID int) error {
	_, err := d.pool.Exec(ctx,
		`UPDATE platform.accounts SET node_id = $1 WHERE id = $2`,
		nodeID, accountID)
	return err
}

// ClaimAvailableNode atomically claims the next available node from the pool
// and binds it to the account. Returns the node_id (nodeutil.URL index), or 0
// if the pool is empty. Also updates platform.accounts.node_id.
func (d *DB) ClaimAvailableNode(ctx context.Context, accountID int64) (int, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var nodeID int
	err = tx.QueryRow(ctx, `
		SELECT id FROM platform.nodes
		WHERE status = 'available'
		ORDER BY id LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(&nodeID)
	if err == pgx.ErrNoRows {
		return 0, nil // pool empty — caller stays on shared node
	}
	if err != nil {
		return 0, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE platform.nodes SET status = 'assigned' WHERE id = $1`, nodeID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE platform.accounts SET node_id = $1 WHERE id = $2`, nodeID, accountID); err != nil {
		return 0, err
	}
	return nodeID, tx.Commit(ctx)
}

// SavePendingSignContext stores a pending signature context created when a
// non-custodial DID deploy returns "Signature needed" from the node.
func (d *DB) SavePendingSignContext(ctx context.Context, signID, action, refID string, accountID int64) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO platform.pending_sign_contexts (sign_id, action, ref_id, account_id)
		VALUES ($1, $2, $3, $4)
	`, signID, action, refID, accountID)
	return err
}

// PopPendingSignContext atomically deletes and returns a pending sign context.
// Returns nil (no error) if the sign_id is not found.
func (d *DB) PopPendingSignContext(ctx context.Context, signID string) (*PendingSignContext, error) {
	p := &PendingSignContext{}
	err := d.pool.QueryRow(ctx, `
		DELETE FROM platform.pending_sign_contexts
		WHERE sign_id = $1
		RETURNING sign_id, action, ref_id, account_id, created_at
	`, signID).Scan(&p.SignID, &p.Action, &p.RefID, &p.AccountID, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (d *DB) SetAccountTier(ctx context.Context, accountID int64, tier, lemonSubID string, nextBilling, cancelAt *time.Time) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE platform.accounts
		SET tier = $1, lemon_subscription_id = $2,
		    next_billing_date = $3, cancel_at = $4
		WHERE id = $5
	`, tier, lemonSubID, nextBilling, cancelAt, accountID)
	return err
}

func scanAccount(row pgx.Row) (*Account, error) {
	a := &Account{}
	err := row.Scan(&a.ID, &a.TelegramID, &a.TelegramUsername, &a.DID, &a.PublicKeyHex,
		&a.NodeID, &a.Tier, &a.LemonSubscriptionID,
		&a.NextBillingDate, &a.CancelAt, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ── Session queries ──────────────────────────────────────────────────────────

func (d *DB) CreateSession(ctx context.Context, accountID int64, token string, expires time.Time) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO platform.sessions (account_id, token, expires_at)
		VALUES ($1, $2, $3)
	`, accountID, token, expires)
	return err
}

func (d *DB) GetSessionAccount(ctx context.Context, token string) (*Account, error) {
	row := d.pool.QueryRow(ctx, `
		SELECT a.id, a.telegram_id, a.telegram_username, COALESCE(a.did,''), COALESCE(a.public_key_hex,''),
		       a.node_id, a.tier, COALESCE(a.lemon_subscription_id,''), a.next_billing_date, a.cancel_at, a.created_at
		FROM platform.sessions s
		JOIN platform.accounts a ON a.id = s.account_id
		WHERE s.token = $1 AND s.expires_at > NOW()
	`, token)
	return scanAccount(row)
}

func (d *DB) DeleteSession(ctx context.Context, token string) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM platform.sessions WHERE token = $1`, token)
	return err
}

func (d *DB) PurgeExpiredSessions(ctx context.Context) error {
	_, err := d.pool.Exec(ctx, `DELETE FROM platform.sessions WHERE expires_at <= NOW()`)
	return err
}

// ── API key queries ──────────────────────────────────────────────────────────

func (d *DB) CreateAPIKey(ctx context.Context, accountID int64, keyHash, label string) (int64, error) {
	var id int64
	err := d.pool.QueryRow(ctx, `
		INSERT INTO platform.api_keys (account_id, key_hash, label)
		VALUES ($1, $2, $3) RETURNING id
	`, accountID, keyHash, label).Scan(&id)
	return id, err
}

func (d *DB) ListAPIKeys(ctx context.Context, accountID int64) ([]APIKey, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, account_id, key_hash, COALESCE(label,''), created_at, revoked_at, last_used_at
		FROM platform.api_keys WHERE account_id = $1 ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.AccountID, &k.KeyHash, &k.Label,
			&k.CreatedAt, &k.RevokedAt, &k.LastUsedAt); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (d *DB) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	k := &APIKey{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, account_id, key_hash, COALESCE(label,''), created_at, revoked_at, last_used_at
		FROM platform.api_keys WHERE key_hash = $1 AND revoked_at IS NULL
	`, hash).Scan(&k.ID, &k.AccountID, &k.KeyHash, &k.Label,
		&k.CreatedAt, &k.RevokedAt, &k.LastUsedAt)
	if err != nil {
		return nil, err
	}
	return k, nil
}

func (d *DB) RevokeAPIKey(ctx context.Context, id, accountID int64) error {
	tag, err := d.pool.Exec(ctx, `
		UPDATE platform.api_keys SET revoked_at = NOW()
		WHERE id = $1 AND account_id = $2 AND revoked_at IS NULL
	`, id, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("key not found or already revoked")
	}
	return nil
}

func (d *DB) TouchAPIKeyLastUsed(ctx context.Context, id int64) {
	// Fire-and-forget; errors are silently dropped.
	d.pool.Exec(context.Background(), //nolint
		`UPDATE platform.api_keys SET last_used_at = NOW() WHERE id = $1`, id)
}

// ── Usage queries ────────────────────────────────────────────────────────────

const FreeTierLimit = 10_000

func (d *DB) IncrementUsage(ctx context.Context, accountID int64) {
	month := time.Now().UTC().Format("2006-01")
	d.pool.Exec(context.Background(), `
		INSERT INTO platform.account_usage (account_id, month, request_count)
		VALUES ($1, $2, 1)
		ON CONFLICT (account_id, month) DO UPDATE
		  SET request_count = platform.account_usage.request_count + 1
	`, accountID, month)
}

func (d *DB) GetUsage(ctx context.Context, accountID int64) (*Usage, error) {
	month := time.Now().UTC().Format("2006-01")
	u := &Usage{AccountID: accountID, Month: month}
	err := d.pool.QueryRow(ctx, `
		SELECT request_count FROM platform.account_usage
		WHERE account_id = $1 AND month = $2
	`, accountID, month).Scan(&u.RequestCount)
	if err == pgx.ErrNoRows {
		return u, nil // zero usage this month
	}
	return u, err
}

// ── Webhook queries ──────────────────────────────────────────────────────────

func (d *DB) CreateWebhookSubscription(ctx context.Context, s *WebhookSubscription) (int64, error) {
	var id int64
	err := d.pool.QueryRow(ctx, `
		INSERT INTO platform.webhook_subscriptions
		  (account_id, event_type, filter_value, callback_url, secret)
		VALUES ($1,$2,$3,$4,$5) RETURNING id
	`, s.AccountID, s.EventType, s.FilterValue, s.CallbackURL, s.Secret).Scan(&id)
	return id, err
}

func (d *DB) ListWebhookSubscriptions(ctx context.Context, accountID int64) ([]WebhookSubscription, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, account_id, event_type, filter_value, callback_url, secret, active, created_at
		FROM platform.webhook_subscriptions WHERE account_id = $1 ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []WebhookSubscription
	for rows.Next() {
		var s WebhookSubscription
		if err := rows.Scan(&s.ID, &s.AccountID, &s.EventType,
			&s.FilterValue, &s.CallbackURL, &s.Secret, &s.Active, &s.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func (d *DB) AllActiveSubscriptions(ctx context.Context) ([]WebhookSubscription, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, account_id, event_type, filter_value, callback_url, secret, active, created_at
		FROM platform.webhook_subscriptions WHERE active = TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subs []WebhookSubscription
	for rows.Next() {
		var s WebhookSubscription
		if err := rows.Scan(&s.ID, &s.AccountID, &s.EventType,
			&s.FilterValue, &s.CallbackURL, &s.Secret, &s.Active, &s.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func (d *DB) DeleteWebhookSubscription(ctx context.Context, id, accountID int64) error {
	tag, err := d.pool.Exec(ctx, `
		DELETE FROM platform.webhook_subscriptions WHERE id = $1 AND account_id = $2
	`, id, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("subscription not found")
	}
	return nil
}

func (d *DB) InsertDelivery(ctx context.Context, del *WebhookDelivery) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO platform.webhook_deliveries
		  (subscription_id, transaction_id, status, response_code)
		VALUES ($1, $2, $3, $4)
	`, del.SubscriptionID, del.TransactionID, del.Status, del.ResponseCode)
	return err
}

func (d *DB) ListDeliveries(ctx context.Context, subscriptionID, accountID int64) ([]WebhookDelivery, error) {
	// Verify ownership via join before returning deliveries.
	rows, err := d.pool.Query(ctx, `
		SELECT d.id, d.subscription_id, d.transaction_id, d.attempted_at, d.status, d.response_code
		FROM platform.webhook_deliveries d
		JOIN platform.webhook_subscriptions s ON s.id = d.subscription_id
		WHERE d.subscription_id = $1 AND s.account_id = $2
		ORDER BY d.attempted_at DESC LIMIT 100
	`, subscriptionID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dels []WebhookDelivery
	for rows.Next() {
		var del WebhookDelivery
		if err := rows.Scan(&del.ID, &del.SubscriptionID, &del.TransactionID,
			&del.AttemptedAt, &del.Status, &del.ResponseCode); err != nil {
			return nil, err
		}
		dels = append(dels, del)
	}
	return dels, rows.Err()
}

// ── Webhook cursor ───────────────────────────────────────────────────────────

func (d *DB) GetWebhookCursor(ctx context.Context) (time.Time, error) {
	var last time.Time
	err := d.pool.QueryRow(ctx,
		`SELECT last_seen FROM platform.webhook_cursor WHERE id = 1`).Scan(&last)
	if err == pgx.ErrNoRows {
		// First start: no cursor row yet. Initialize to now and return zero time
		// so the worker starts from "no prior transactions seen".
		if _, initErr := d.pool.Exec(ctx,
			`INSERT INTO platform.webhook_cursor (id, last_seen) VALUES (1, NOW()) ON CONFLICT (id) DO NOTHING`,
		); initErr != nil {
			return time.Time{}, initErr
		}
		return time.Time{}, nil
	}
	return last, err
}

// GetDataSince returns the oldest created_at timestamp in fullnode_transactions,
// which is the earliest point from which the fullnode's coverage is continuous.
// Returns zero Time if the table is empty or unreachable.
func (d *DB) GetDataSince(ctx context.Context) (time.Time, error) {
	// MIN() on an empty table returns SQL NULL; scan into *time.Time to handle it.
	var t *time.Time
	err := d.rubixPool.QueryRow(ctx,
		`SELECT MIN(created_at) FROM fullnode_transactions`).Scan(&t)
	if err != nil || t == nil {
		return time.Time{}, err
	}
	return *t, nil
}

// FreeNode releases a dedicated node back to the pool when a paid subscription
// expires or is cancelled. The account's node_id is reset to 0 (shared node).
// No-op if the account is already on the shared node (node_id = 0).
func (d *DB) FreeNode(ctx context.Context, accountID int64) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var nodeID int
	if err := tx.QueryRow(ctx,
		`SELECT node_id FROM platform.accounts WHERE id = $1 FOR UPDATE`, accountID).Scan(&nodeID); err != nil {
		return err
	}
	if nodeID == 0 {
		return nil // already on shared node, nothing to free
	}

	if _, err := tx.Exec(ctx,
		`UPDATE platform.nodes SET status = 'available' WHERE id = $1`, nodeID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE platform.accounts SET node_id = 0 WHERE id = $1`, accountID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (d *DB) SetWebhookCursor(ctx context.Context, lastSeen time.Time) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO platform.webhook_cursor (id, last_seen) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET last_seen = EXCLUDED.last_seen
	`, lastSeen)
	return err
}

func (d *DB) MarkContractDeployed(ctx context.Context, contractID string) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE platform.hosted_contracts
		SET deployed_at = NOW()
		WHERE contract_id = $1 AND deployed_at IS NULL
	`, contractID)
	return err
}

// ── Fullnode transaction polling ─────────────────────────────────────────────

// FullnodeTx is a minimal view of fullnode_transactions needed by the webhook worker.
// The rubixgoplatform schema has id TEXT PRIMARY KEY (the tx hash, not a sequence).
type FullnodeTx struct {
	ID        string // tx hash — also the canonical transaction identifier
	Info      map[string]interface{}
	CreatedAt time.Time
}

// PollNewTransactions returns rows from fullnode_transactions created after afterTime.
// Uses rubixPool — the connection pool pointed at the rubix node's database.
// Configure RUBIX_DATABASE_URL separately from DATABASE_URL in production.
func (d *DB) PollNewTransactions(ctx context.Context, afterTime time.Time) ([]FullnodeTx, error) {
	rows, err := d.rubixPool.Query(ctx, `
		SELECT id, info, created_at
		FROM fullnode_transactions
		WHERE created_at > $1
		ORDER BY created_at ASC, id ASC
		LIMIT 500
	`, afterTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var txns []FullnodeTx
	for rows.Next() {
		var t FullnodeTx
		var infoBytes []byte
		if err := rows.Scan(&t.ID, &infoBytes, &t.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(infoBytes, &t.Info); err != nil {
			t.Info = map[string]interface{}{}
		}
		txns = append(txns, t)
	}
	return txns, rows.Err()
}

// ── Contract queries ─────────────────────────────────────────────────────────

func (d *DB) CreateHostedContract(ctx context.Context, c *HostedContract) (int64, error) {
	var id int64
	err := d.pool.QueryRow(ctx, `
		INSERT INTO platform.hosted_contracts
		  (account_id, contract_id, wasm_artifact_hash, initial_state, current_state)
		VALUES ($1,$2,$3,$4,$5) RETURNING id
	`, c.AccountID, c.ContractID, c.WASMArtifactHash, c.InitialState, c.CurrentState).Scan(&id)
	return id, err
}

func (d *DB) DeleteHostedContract(ctx context.Context, contractID string) error {
	_, err := d.pool.Exec(ctx,
		`DELETE FROM platform.hosted_contracts WHERE contract_id = $1`, contractID)
	return err
}

func (d *DB) GetHostedContractByRubixID(ctx context.Context, contractID string) (*HostedContract, error) {
	c := &HostedContract{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, account_id, contract_id, COALESCE(wasm_artifact_hash,''),
		       initial_state, current_state, deployed_at, execution_count
		FROM platform.hosted_contracts WHERE contract_id = $1
	`, contractID).Scan(&c.ID, &c.AccountID, &c.ContractID, &c.WASMArtifactHash,
		&c.InitialState, &c.CurrentState, &c.DeployedAt, &c.ExecutionCount)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (d *DB) GetHostedContractByRowID(ctx context.Context, rowID, accountID int64) (*HostedContract, error) {
	c := &HostedContract{}
	err := d.pool.QueryRow(ctx, `
		SELECT id, account_id, contract_id, COALESCE(wasm_artifact_hash,''),
		       initial_state, current_state, deployed_at, execution_count
		FROM platform.hosted_contracts WHERE id = $1 AND account_id = $2
	`, rowID, accountID).Scan(&c.ID, &c.AccountID, &c.ContractID, &c.WASMArtifactHash,
		&c.InitialState, &c.CurrentState, &c.DeployedAt, &c.ExecutionCount)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (d *DB) ListHostedContracts(ctx context.Context, accountID int64) ([]HostedContract, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, account_id, contract_id, COALESCE(wasm_artifact_hash,''),
		       initial_state, current_state, deployed_at, execution_count
		FROM platform.hosted_contracts WHERE account_id = $1
		ORDER BY id DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cs []HostedContract
	for rows.Next() {
		var c HostedContract
		if err := rows.Scan(&c.ID, &c.AccountID, &c.ContractID, &c.WASMArtifactHash,
			&c.InitialState, &c.CurrentState, &c.DeployedAt, &c.ExecutionCount); err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	return cs, rows.Err()
}

func (d *DB) UpdateContractState(ctx context.Context, contractID string, newState json.RawMessage) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE platform.hosted_contracts
		SET current_state = $1, execution_count = execution_count + 1
		WHERE contract_id = $2
	`, newState, contractID)
	return err
}

func (d *DB) InsertContractExecution(ctx context.Context, e *ContractExecution) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO platform.contract_executions
		  (contract_id, initiator_did, input, output, state_before, state_after, success, error)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, e.ContractID, e.InitiatorDID, e.Input, e.Output,
		e.StateBefore, e.StateAfter, e.Success, e.Error)
	return err
}

func (d *DB) ListContractExecutions(ctx context.Context, contractID string, accountID int64) ([]ContractExecution, error) {
	// Verify ownership via join.
	rows, err := d.pool.Query(ctx, `
		SELECT e.id, e.contract_id, e.executed_at, COALESCE(e.initiator_did,''),
		       e.input, e.output, e.state_before, e.state_after, e.success, COALESCE(e.error,'')
		FROM platform.contract_executions e
		JOIN platform.hosted_contracts c ON c.contract_id = e.contract_id
		WHERE e.contract_id = $1 AND c.account_id = $2
		ORDER BY e.executed_at DESC LIMIT 100
	`, contractID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var execs []ContractExecution
	for rows.Next() {
		var ex ContractExecution
		if err := rows.Scan(&ex.ID, &ex.ContractID, &ex.ExecutedAt, &ex.InitiatorDID,
			&ex.Input, &ex.Output, &ex.StateBefore, &ex.StateAfter,
			&ex.Success, &ex.Error); err != nil {
			return nil, err
		}
		execs = append(execs, ex)
	}
	return execs, rows.Err()
}
