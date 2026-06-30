// Package gateway handles API key management, quota enforcement, and the
// reverse proxy that forwards write operations to the user's Rubix node.
package gateway

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"node3.cloud/platform/internal/auth"
	"node3.cloud/platform/internal/db"
	"node3.cloud/platform/internal/nodeutil"
)

// Handler owns API key management and the node proxy.
type Handler struct {
	db         *db.DB
	secret     string // used to derive per-account DID passwords; never leaves the server
	client     *http.Client // for short-lived operations (key checks, DID create, tx initiate)
	longClient *http.Client // for consensus operations that can take minutes (tx sign)
}

func NewHandler(database *db.DB, secret string) *Handler {
	return &Handler{
		db:         database,
		secret:     secret,
		client:     &http.Client{Timeout: 30 * time.Second},
		longClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

// ── API key helpers ──────────────────────────────────────────────────────────

// generateAPIKey returns a new key and its SHA-256 hash.
// Format: n3k_<64 hex chars>
func generateAPIKey() (key, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	key = hex.EncodeToString(b)
	h := sha256.Sum256([]byte(key))
	hash = hex.EncodeToString(h[:])
	return
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// ── HTTP handlers ────────────────────────────────────────────────────────────

func (h *Handler) HandleListKeys(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	keys, err := h.db.ListAPIKeys(r.Context(), account.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]keyResponse, 0, len(keys))
	for _, k := range keys {
		out = append(out, keyToResponse(k))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) HandleCreateKey(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())

	if account.Tier == "free" {
		keys, err := h.db.ListAPIKeys(r.Context(), account.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "could not check key limit")
			return
		}
		active := 0
		for _, k := range keys {
			if k.RevokedAt == nil {
				active++
			}
		}
		if active >= 1 {
			writeErr(w, http.StatusForbidden, "free tier limit: 1 active API key")
			return
		}
	}

	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	key, hash, err := generateAPIKey()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "key generation failed")
		return
	}

	id, err := h.db.CreateAPIKey(r.Context(), account.ID, hash, body.Label)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"id":  strconv.FormatInt(id, 10),
		"key": key,
	})
}

func (h *Handler) HandleRevokeKey(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid key id")
		return
	}
	if err := h.db.RevokeAPIKey(r.Context(), id, account.ID); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) HandleGetUsage(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	usage, err := h.db.GetUsage(r.Context(), account.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	limit := db.FreeTierLimit
	if account.Tier == "paid" {
		limit = 10_000_000 // effectively unlimited
	}

	now := time.Now().UTC()
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)

	// data_since tells callers the earliest point from which fullnode_transactions
	// coverage is guaranteed (forward-only: the moment the fullnode started indexing).
	dataSince, _ := h.db.GetDataSince(r.Context())

	resp := map[string]interface{}{
		"request_count": usage.RequestCount,
		"limit":         limit,
		"month":         usage.Month,
		"reset_at":      nextMonth.Format(time.RFC3339),
	}
	if !dataSince.IsZero() {
		resp["data_since"] = dataSince.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleCreateDID creates a DID on the user's assigned node.
// If public_key (65-byte uncompressed secp256k1 hex) is provided the node
// uses it directly (non-custodial). If omitted the node generates a fresh
// mnemonic and keypair internally.
func (h *Handler) HandleCreateDID(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	if account.DID != "" {
		writeErr(w, http.StatusConflict, "DID already exists for this account")
		return
	}

	var body struct {
		PublicKey string `json:"public_key"`
		Password  string `json:"password"`
	}
	// Ignore decode errors — empty body is valid (node generates keypair).
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck

	// Derive a per-account password from the server secret so it is neither
	// user-controlled nor predictable from public data (account IDs are sequential).
	password := body.Password
	if password == "" {
		mac := hmac.New(sha256.New, []byte(h.secret))
		mac.Write([]byte(strconv.FormatInt(account.ID, 10)))
		password = hex.EncodeToString(mac.Sum(nil))
	}

	nodePayload := map[string]string{"password": password}
	if body.PublicKey != "" {
		nodePayload["public_key"] = body.PublicKey
	}

	reqBody, _ := json.Marshal(nodePayload)
	resp, err := h.client.Post(nodeutil.URL(account.NodeID)+"/rubix/v1/dids/create",
		"application/json", bytes.NewReader(reqBody))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "node unreachable")
		return
	}
	defer resp.Body.Close()

	// Node responds with BasicResponse{status, message, result: {did, peer_id}}
	var result struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Result  struct {
			DID    string `json:"did"`
			PeerID string `json:"peer_id"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		writeErr(w, http.StatusBadGateway, "invalid response from node")
		return
	}
	if !result.Status || result.Result.DID == "" {
		writeErr(w, http.StatusBadGateway, "node error: "+result.Message)
		return
	}

	if err := h.db.SetAccountDID(r.Context(), account.ID, result.Result.DID); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to persist DID")
		return
	}
	if body.PublicKey != "" {
		// Best-effort; non-fatal if this fails since the DID is already created.
		h.db.SetAccountPublicKey(r.Context(), account.ID, body.PublicKey) //nolint:errcheck
	}

	writeJSON(w, http.StatusOK, map[string]string{"did": result.Result.DID})
}

// ── Transaction relay (non-custodial signing) ────────────────────────────────

// HandleTxInitiate forwards a transaction initiation to the user's node.
// For non-custodial DIDs the node immediately returns "Signature needed" with
// {id, hash}. The browser signs the hash and calls HandleTxSign to complete.
func (h *Handler) HandleTxInitiate(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	resp, err := h.client.Post(
		nodeutil.URL(account.NodeID)+"/rubix/v1/tx",
		"application/json", r.Body)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "node unreachable")
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

// HandleTxSign forwards the browser's signature to the node, completing
// consensus. If a pending deploy context exists for this sign_id, marks
// the contract deployed once consensus succeeds.
func (h *Handler) HandleTxSign(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())

	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read error")
		return
	}

	// Peek at the sign_id so we can check pending contexts after consensus.
	var req struct {
		ID string `json:"id"`
	}
	json.Unmarshal(body, &req) //nolint:errcheck

	resp, err := h.longClient.Post(
		nodeutil.URL(account.NodeID)+"/rubix/v1/signature",
		"application/json", bytes.NewReader(body))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "node unreachable")
		return
	}
	defer resp.Body.Close()

	// Read the full response so we can inspect status before forwarding.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "failed to read node response")
		return
	}

	// On consensus success, fire any pending side-effects (e.g. mark contract deployed).
	var nodeResult struct {
		Status bool `json:"status"`
	}
	if req.ID != "" && json.Unmarshal(respBody, &nodeResult) == nil && nodeResult.Status {
		if psc, err := h.db.PopPendingSignContext(r.Context(), req.ID); err == nil && psc != nil {
			if psc.Action == "deploy" {
				h.db.MarkContractDeployed(context.Background(), psc.RefID) //nolint:errcheck
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody) //nolint:errcheck
}

// CreateDIDOnNode creates a DID on the node assigned to account. For non-custodial
// accounts (PublicKeyHex set), registers using the existing public key. Falls back to
// a server-derived HMAC password if no public key is present.
func (h *Handler) CreateDIDOnNode(ctx context.Context, account *db.Account) (string, error) {
	nodePayload := map[string]string{}
	if account.PublicKeyHex != "" {
		nodePayload["public_key"] = account.PublicKeyHex
	} else {
		mac := hmac.New(sha256.New, []byte(h.secret))
		mac.Write([]byte(strconv.FormatInt(account.ID, 10)))
		nodePayload["password"] = hex.EncodeToString(mac.Sum(nil))
	}

	reqBody, _ := json.Marshal(nodePayload)
	resp, err := h.client.Post(
		nodeutil.URL(account.NodeID)+"/rubix/v1/dids/create",
		"application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("node unreachable: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Result  struct {
			DID string `json:"did"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("invalid node response: %w", err)
	}
	if !result.Status || result.Result.DID == "" {
		return "", fmt.Errorf("node error: %s", result.Message)
	}
	return result.Result.DID, nil
}

// ── Node proxy (for future /rubix/* pass-through) ────────────────────────────

// HandleNodeProxy is a middleware-compatible handler that forwards the request
// to the rubixgoplatform node assigned to the authenticated account.
// The account must already be in context (set by RequireAPIKey or RequireSession).
//
// A new reverse proxy is constructed per unique node URL. For the current
// single-node deployment this is called with node_id=0 every time; the
// allocation cost is negligible. Cache proxies by node_id if node count grows.
func (h *Handler) HandleNodeProxy(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	rawURL := nodeutil.URL(account.NodeID)
	target, err := url.Parse(rawURL)
	if err != nil || target.Host == "" {
		writeErr(w, http.StatusInternalServerError, "invalid node configuration")
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		writeErr(w, http.StatusBadGateway, "node unreachable")
	}

	// For read requests, inject X-Data-Since so callers know the earliest
	// timestamp from which fullnode coverage is continuous (forward-only).
	if r.Method == http.MethodGet {
		if since, err := h.db.GetDataSince(r.Context()); err == nil && !since.IsZero() {
			proxy.ModifyResponse = func(resp *http.Response) error {
				resp.Header.Set("X-Data-Since", since.UTC().Format(time.RFC3339))
				return nil
			}
		}
	}

	proxy.ServeHTTP(w, r)
}

// RequireAPIKey middleware authenticates requests using a hashed API key.
// Injects the account into context and increments usage asynchronously.
func (h *Handler) RequireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := apiKeyFromRequest(r)
		if raw == "" {
			writeErr(w, http.StatusUnauthorized, "missing API key")
			return
		}
		hash := hashKey(raw)
		k, err := h.db.GetAPIKeyByHash(r.Context(), hash)
		if err != nil || k.RevokedAt != nil {
			writeErr(w, http.StatusUnauthorized, "invalid or revoked key")
			return
		}

		account, err := h.db.GetAccountByID(r.Context(), k.AccountID)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "account not found")
			return
		}

		// Quota check for free tier.
		if account.Tier == "free" {
			usage, _ := h.db.GetUsage(r.Context(), account.ID)
			if usage != nil && usage.RequestCount >= db.FreeTierLimit {
				now := time.Now().UTC()
				reset := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
				w.Header().Set("Retry-After", reset.Format(time.RFC1123))
				writeErr(w, http.StatusTooManyRequests, "monthly quota exceeded — upgrade to Pro")
				return
			}
		}

		go h.db.TouchAPIKeyLastUsed(context.Background(), k.ID)
		go h.db.IncrementUsage(context.Background(), account.ID)

		ctx := auth.WithAccount(r.Context(), account)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func apiKeyFromRequest(r *http.Request) string {
	if v := r.Header.Get("X-API-Key"); v != "" {
		return v
	}
	v := r.Header.Get("Authorization")
	if strings.HasPrefix(v, "Bearer ") {
		return strings.TrimPrefix(v, "Bearer ")
	}
	return ""
}

// ── Response helpers ─────────────────────────────────────────────────────────

type keyResponse struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	CreatedAt  string  `json:"created_at"`
	RevokedAt  *string `json:"revoked_at"`
	LastUsedAt *string `json:"last_used_at"`
}

func keyToResponse(k db.APIKey) keyResponse {
	r := keyResponse{
		ID:        strconv.FormatInt(k.ID, 10),
		Label:     k.Label,
		CreatedAt: k.CreatedAt.Format(time.RFC3339),
	}
	if k.RevokedAt != nil {
		s := k.RevokedAt.Format(time.RFC3339)
		r.RevokedAt = &s
	}
	if k.LastUsedAt != nil {
		s := k.LastUsedAt.Format(time.RFC3339)
		r.LastUsedAt = &s
	}
	return r
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
