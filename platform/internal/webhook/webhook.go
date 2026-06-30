// Package webhook manages developer webhook subscriptions and the background
// worker that tails fullnode_transactions and fires HTTP callbacks on matches.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"node3.cloud/platform/internal/auth"
	"node3.cloud/platform/internal/db"
)

// Handler owns webhook subscription CRUD.
type Handler struct {
	db *db.DB
}

func NewHandler(database *db.DB) *Handler {
	return &Handler{db: database}
}

// ── HTTP handlers ────────────────────────────────────────────────────────────

func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	subs, err := h.db.ListWebhookSubscriptions(r.Context(), account.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]subResponse, 0, len(subs))
	for _, s := range subs {
		out = append(out, subToResponse(s))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())

	// Free tier limit: 3 active subscriptions. Fail closed on DB error.
	if account.Tier == "free" {
		subs, err := h.db.ListWebhookSubscriptions(r.Context(), account.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "could not check webhook limit")
			return
		}
		active := 0
		for _, s := range subs {
			if s.Active {
				active++
			}
		}
		if active >= 3 {
			writeErr(w, http.StatusForbidden, "free tier limit: 3 active webhooks")
			return
		}
	}

	var body struct {
		EventType   string `json:"event_type"`
		FilterValue string `json:"filter_value"`
		CallbackURL string `json:"callback_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.EventType == "" || body.FilterValue == "" || body.CallbackURL == "" {
		writeErr(w, http.StatusBadRequest, "event_type, filter_value, callback_url required")
		return
	}

	secret, err := generateSecret()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to generate secret")
		return
	}

	sub := &db.WebhookSubscription{
		AccountID:   account.ID,
		EventType:   body.EventType,
		FilterValue: body.FilterValue,
		CallbackURL: body.CallbackURL,
		Secret:      secret,
	}

	id, err := h.db.CreateWebhookSubscription(r.Context(), sub)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	sub.ID = id
	sub.Active = true
	sub.CreatedAt = time.Now()

	writeJSON(w, http.StatusOK, subToResponse(*sub))
}

func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.db.DeleteWebhookSubscription(r.Context(), id, account.ID); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) HandleDeliveries(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	subID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	dels, err := h.db.ListDeliveries(r.Context(), subID, account.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]deliveryResponse, 0, len(dels))
	for _, d := range dels {
		out = append(out, deliveryToResponse(d))
	}
	writeJSON(w, http.StatusOK, out)
}

// ── Worker ───────────────────────────────────────────────────────────────────

// Worker polls fullnode_transactions and fans out to matching webhook subscribers.
type Worker struct {
	db       *db.DB
	interval time.Duration
	client   *http.Client
}

func NewWorker(database *db.DB, pollInterval time.Duration) *Worker {
	return &Worker{
		db:       database,
		interval: pollInterval,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Start runs the poll loop until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	log.Printf("webhook worker started, polling every %s", w.interval)
	for {
		select {
		case <-ctx.Done():
			log.Println("webhook worker stopped")
			return
		case <-ticker.C:
			if err := w.poll(ctx); err != nil {
				log.Printf("webhook worker poll error: %v", err)
			}
		}
	}
}

func (w *Worker) poll(ctx context.Context) error {
	cursor, err := w.db.GetWebhookCursor(ctx)
	if err != nil {
		return fmt.Errorf("get cursor: %w", err)
	}

	txns, err := w.db.PollNewTransactions(ctx, cursor)
	if err != nil {
		return fmt.Errorf("poll transactions: %w", err)
	}
	if len(txns) == 0 {
		return nil
	}

	subs, err := w.db.AllActiveSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("list subscriptions: %w", err)
	}
	if len(subs) == 0 {
		// Advance cursor even if no subscribers.
		return w.db.SetWebhookCursor(ctx, txns[len(txns)-1].CreatedAt)
	}

	var wg sync.WaitGroup
	for _, tx := range txns {
		for i := range subs {
			if matched, event := matchTx(tx, &subs[i]); matched {
				wg.Add(1)
				sub := subs[i] // copy before goroutine captures it
				txID := tx.ID
				go func() {
					defer wg.Done()
					w.deliver(sub, txID, event)
				}()
			}
		}
	}
	wg.Wait() // advance cursor only after all deliveries complete or time out

	return w.db.SetWebhookCursor(ctx, txns[len(txns)-1].CreatedAt)
}

// matchTx returns true and the event payload if the transaction matches the subscription.
//
// Field paths are derived from rubixgoplatform's TransactionInfo struct
// (types/models/transaction_info.go), which is what fullnode_transactions.info stores:
//
//	info["initiator"]  — sender DID
//	info["owner"]      — receiver DID
//	info["tokens"]["smartContract"][0]["tokenId"]               — contract token ID (Qm...)
//	info["tokens"]["smartContract"][0]["previousTransactionID"] — "" on deploy, non-empty on execute
func matchTx(tx db.FullnodeTx, sub *db.WebhookSubscription) (bool, map[string]interface{}) {
	info := tx.Info

	strVal := func(key string) string {
		v, _ := info[key].(string)
		return v
	}

	switch sub.EventType {
	case "token.received":
		if strVal("owner") != sub.FilterValue {
			return false, nil
		}
		return true, buildEvent("token.received", tx)

	case "token.sent":
		if strVal("initiator") != sub.FilterValue {
			return false, nil
		}
		return true, buildEvent("token.sent", tx)

	case "contract.deployed":
		tokenID, prevTxID := scTokenInfo(info)
		if tokenID != sub.FilterValue || prevTxID != "" {
			return false, nil
		}
		return true, buildEvent("contract.deployed", tx)

	case "contract.executed":
		tokenID, prevTxID := scTokenInfo(info)
		if tokenID != sub.FilterValue || prevTxID == "" {
			return false, nil
		}
		return true, buildEvent("contract.executed", tx)
	}

	return false, nil
}

// scTokenInfo navigates info["tokens"]["smartContract"][0] and returns
// the token ID and previousTransactionID. Both are empty string if absent.
func scTokenInfo(info map[string]interface{}) (tokenID, prevTxID string) {
	tokens, _ := info["tokens"].(map[string]interface{})
	if tokens == nil {
		return
	}
	scList, _ := tokens["smartContract"].([]interface{})
	if len(scList) == 0 {
		return
	}
	sc0, _ := scList[0].(map[string]interface{})
	if sc0 == nil {
		return
	}
	tokenID, _ = sc0["tokenId"].(string)
	prevTxID, _ = sc0["previousTransactionID"].(string)
	return
}

func buildEvent(eventType string, tx db.FullnodeTx) map[string]interface{} {
	return map[string]interface{}{
		"event":          eventType,
		"transaction_id": tx.ID,
		"timestamp":      tx.CreatedAt.Format(time.RFC3339),
		"data":           tx.Info,
	}
}

// deliver POSTs the event payload to the subscriber's callback URL.
// Retries up to 3 times with exponential backoff (1s, 2s, 4s) on failure.
// Uses its own timeout context so a server shutdown does not cancel
// in-flight deliveries mid-write (which would produce connection resets
// on the subscriber's end with no way to distinguish from a network error).
func (w *Worker) deliver(sub db.WebhookSubscription, txID string, payload map[string]interface{}) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	sig := sign(body, sub.Secret)

	const maxAttempts = 3
	backoff := time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), w.client.Timeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.CallbackURL, bytes.NewReader(body))
		if err != nil {
			cancel()
			w.recordDelivery(sub.ID, txID, "failed", nil)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Rubix-Signature", "sha256="+sig)
		req.Header.Set("X-Rubix-Event", sub.EventType)

		resp, err := w.client.Do(req)
		cancel()

		if err == nil {
			code := resp.StatusCode
			resp.Body.Close()
			if code >= 200 && code < 300 {
				w.recordDelivery(sub.ID, txID, "success", &code)
				return
			}
			// Non-2xx: record and retry if attempts remain.
			if attempt == maxAttempts {
				w.recordDelivery(sub.ID, txID, "failed", &code)
				return
			}
		} else if attempt == maxAttempts {
			w.recordDelivery(sub.ID, txID, "failed", nil)
			return
		}

		time.Sleep(backoff)
		backoff *= 2
	}
}

func (w *Worker) recordDelivery(subID int64, txID, status string, code *int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	w.db.InsertDelivery(ctx, &db.WebhookDelivery{
		SubscriptionID: subID,
		TransactionID:  txID,
		Status:         status,
		ResponseCode:   code,
	})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ── Response types ───────────────────────────────────────────────────────────

type subResponse struct {
	ID          string `json:"id"`
	EventType   string `json:"event_type"`
	FilterValue string `json:"filter_value"`
	CallbackURL string `json:"callback_url"`
	Active      bool   `json:"active"`
	CreatedAt   string `json:"created_at"`
}

type deliveryResponse struct {
	ID             string  `json:"id"`
	SubscriptionID string  `json:"subscription_id"`
	TransactionID  string  `json:"transaction_id"`
	AttemptedAt    string  `json:"attempted_at"`
	Status         string  `json:"status"`
	ResponseCode   *int    `json:"response_code"`
}

func subToResponse(s db.WebhookSubscription) subResponse {
	return subResponse{
		ID:          strconv.FormatInt(s.ID, 10),
		EventType:   s.EventType,
		FilterValue: s.FilterValue,
		CallbackURL: s.CallbackURL,
		Active:      s.Active,
		CreatedAt:   s.CreatedAt.Format(time.RFC3339),
	}
}

func deliveryToResponse(d db.WebhookDelivery) deliveryResponse {
	return deliveryResponse{
		ID:             strconv.FormatInt(d.ID, 10),
		SubscriptionID: strconv.FormatInt(d.SubscriptionID, 10),
		TransactionID:  d.TransactionID,
		AttemptedAt:    d.AttemptedAt.Format(time.RFC3339),
		Status:         d.Status,
		ResponseCode:   d.ResponseCode,
	}
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
