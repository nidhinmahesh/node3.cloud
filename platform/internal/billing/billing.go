// Package billing handles Lemon Squeezy webhook verification and subscription
// lifecycle (subscription_created, subscription_cancelled, payment_failed).
package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"node3.cloud/platform/internal/auth"
	"node3.cloud/platform/internal/db"
	"node3.cloud/platform/internal/gateway"
)

// Handler owns billing endpoints and Lemon Squeezy webhook processing.
type Handler struct {
	db              *db.DB
	gw              *gateway.Handler // used to create DID on dedicated node after upgrade
	lemonSecret     string           // Lemon Squeezy webhook signing secret
	lemonAPIKey     string           // Lemon Squeezy API key for creating checkout sessions
	lemonVariantID  string           // Product variant ID for the Pro plan
	platformBaseURL string           // e.g. https://node3.cloud
	client          *http.Client
}

func NewHandler(database *db.DB, gw *gateway.Handler, lemonSecret, lemonAPIKey, lemonVariantID, platformBaseURL string) *Handler {
	return &Handler{
		db:              database,
		gw:              gw,
		lemonSecret:     lemonSecret,
		lemonAPIKey:     lemonAPIKey,
		lemonVariantID:  lemonVariantID,
		platformBaseURL: platformBaseURL,
		client:          &http.Client{Timeout: 15 * time.Second},
	}
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

func (h *Handler) HandleGetBilling(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tier":              account.Tier,
		"subscription_id":   nilIfEmpty(account.LemonSubscriptionID),
		"next_billing_date": fmtTimePtr(account.NextBillingDate),
		"cancel_at":         fmtTimePtr(account.CancelAt),
	})
}

// HandleCheckout creates a Lemon Squeezy checkout session and returns the URL.
func (h *Handler) HandleCheckout(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	if account.Tier == "paid" {
		writeErr(w, http.StatusConflict, "already on Pro plan")
		return
	}

	checkoutURL, err := h.createCheckoutURL(account)
	if err != nil {
		writeErr(w, http.StatusBadGateway, fmt.Sprintf("checkout: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": checkoutURL})
}

// HandleCancel schedules cancellation via Lemon Squeezy API.
func (h *Handler) HandleCancel(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	if account.Tier != "paid" || account.LemonSubscriptionID == "" {
		writeErr(w, http.StatusBadRequest, "no active subscription")
		return
	}
	if err := h.cancelSubscription(account.LemonSubscriptionID); err != nil {
		writeErr(w, http.StatusBadGateway, fmt.Sprintf("cancel: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// HandleLemonWebhook receives and processes Lemon Squeezy webhook events.
func (h *Handler) HandleLemonWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read error")
		return
	}

	if !h.verifySignature(r.Header.Get("X-Signature"), body) {
		writeErr(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	var event struct {
		Meta struct {
			EventName string `json:"event_name"`
		} `json:"meta"`
		Data struct {
			ID         string `json:"id"` // subscription ID
			Attributes struct {
				Status          string  `json:"status"`
				CustomerEmail   string  `json:"customer_email"`
				RenewsAt        *string `json:"renews_at"`
				EndsAt          *string `json:"ends_at"`
				UserEmail       string  `json:"user_email"`
				CustomData      struct {
					AccountID string `json:"account_id"`
				} `json:"custom_data"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &event); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	subID := event.Data.ID
	attrs := event.Data.Attributes
	accountIDStr := attrs.CustomData.AccountID

	accountID, _ := strconv.ParseInt(accountIDStr, 10, 64)
	if accountID == 0 {
		// Try to find account by existing lemon subscription ID.
		if acc, err := h.db.GetAccountByLemonID(r.Context(), subID); err == nil {
			accountID = acc.ID
		}
	}

	if accountID == 0 {
		// Return 500 so Lemon Squeezy retries — a 200 would mark delivery as
		// successful and the subscription would never be provisioned.
		log.Printf("billing webhook: could not resolve account for sub %s event %s", subID, event.Meta.EventName)
		writeErr(w, http.StatusInternalServerError, "account not found")
		return
	}

	switch event.Meta.EventName {
	case "subscription_created", "subscription_updated":
		var nextBilling *time.Time
		if attrs.RenewsAt != nil {
			t, err := time.Parse(time.RFC3339, *attrs.RenewsAt)
			if err == nil {
				nextBilling = &t
			}
		}
		if err := h.db.SetAccountTier(r.Context(), accountID, "paid", subID, nextBilling, nil); err != nil {
			log.Printf("billing: SetAccountTier failed for account %d event %s: %v", accountID, event.Meta.EventName, err)
			writeErr(w, http.StatusInternalServerError, "failed to update account tier")
			return
		}

		// Provision dedicated node on first upgrade (subscription_created only).
		// subscription_updated fires on renewals — skip re-provisioning then.
		if event.Meta.EventName == "subscription_created" {
			nodeID, err := h.db.ClaimAvailableNode(r.Context(), accountID)
			if err != nil || nodeID == 0 {
				log.Printf("billing: no dedicated node available for account %d (err=%v) — staying on shared node", accountID, err)
			} else {
				account, err := h.db.GetAccountByID(r.Context(), accountID)
				if err != nil {
					// Node claimed but account fetch failed — release the node to avoid orphaning it.
					log.Printf("billing: GetAccountByID failed after node claim for account %d: %v", accountID, err)
					if freeErr := h.db.FreeNode(r.Context(), accountID); freeErr != nil {
						log.Printf("billing: failed to release orphaned node for account %d: %v", accountID, freeErr)
					}
				} else {
					did, err := h.gw.CreateDIDOnNode(r.Context(), account)
					if err != nil {
						log.Printf("billing: DID creation on node %d failed for account %d: %v", nodeID, accountID, err)
					} else {
						h.db.SetAccountDID(r.Context(), accountID, did) //nolint:errcheck
					}
				}
			}
		}

	case "subscription_cancelled":
		var cancelAt *time.Time
		if attrs.EndsAt != nil {
			t, err := time.Parse(time.RFC3339, *attrs.EndsAt)
			if err == nil {
				cancelAt = &t
			}
		}
		// Keep "paid" until the billing period ends; subscription_expired fires then.
		if err := h.db.SetAccountTier(r.Context(), accountID, "paid", subID, nil, cancelAt); err != nil {
			log.Printf("billing: SetAccountTier failed for account %d event %s: %v", accountID, event.Meta.EventName, err)
			writeErr(w, http.StatusInternalServerError, "failed to update account tier")
			return
		}

	case "subscription_expired":
		// Billing period has ended after cancellation — downgrade and release node.
		if err := h.db.SetAccountTier(r.Context(), accountID, "free", "", nil, nil); err != nil {
			log.Printf("billing: SetAccountTier failed for account %d event %s: %v", accountID, event.Meta.EventName, err)
			writeErr(w, http.StatusInternalServerError, "failed to downgrade account")
			return
		}
		if err := h.db.FreeNode(r.Context(), accountID); err != nil {
			log.Printf("billing: failed to free node for account %d: %v", accountID, err)
		}

	case "subscription_payment_failed":
		// Grace period — Lemon Squeezy retries; subscription_expired fires if
		// retries exhaust, which triggers the downgrade above.
	}

	w.WriteHeader(http.StatusOK)
}

// ── Lemon Squeezy API calls ───────────────────────────────────────────────────

// createCheckoutURL calls the Lemon Squeezy API to create a checkout session
// and returns the URL the user should be redirected to.
func (h *Handler) createCheckoutURL(account *db.Account) (string, error) {
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "checkouts",
			"attributes": map[string]interface{}{
				"checkout_options": map[string]interface{}{
					"embed": false,
				},
				"checkout_data": map[string]interface{}{
					"custom": map[string]string{
						"account_id": strconv.FormatInt(account.ID, 10),
					},
				},
				"product_options": map[string]interface{}{
					"redirect_url": h.platformBaseURL + "/billing?upgraded=1",
				},
			},
			"relationships": map[string]interface{}{
				"variant": map[string]interface{}{
					"data": map[string]string{
						"type": "variants",
						"id":   h.lemonVariantID,
					},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, "https://api.lemonsqueezy.com/v1/checkouts",
		strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+h.lemonAPIKey)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Accept", "application/vnd.api+json")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Attributes struct {
				URL string `json:"url"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Data.Attributes.URL == "" {
		return "", fmt.Errorf("empty checkout URL from Lemon Squeezy")
	}
	return result.Data.Attributes.URL, nil
}

func (h *Handler) cancelSubscription(subID string) error {
	req, err := http.NewRequest(http.MethodDelete,
		"https://api.lemonsqueezy.com/v1/subscriptions/"+subID, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+h.lemonAPIKey)
	req.Header.Set("Accept", "application/vnd.api+json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("lemon squeezy returned %d", resp.StatusCode)
	}
	return nil
}

// verifySignature checks the Lemon Squeezy HMAC-SHA256 webhook signature.
func (h *Handler) verifySignature(sigHeader string, body []byte) bool {
	mac := hmac.New(sha256.New, []byte(h.lemonSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sigHeader))
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func fmtTimePtr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
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
