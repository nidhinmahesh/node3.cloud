// Package auth handles Telegram Login Widget verification and session management.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"node3.cloud/platform/internal/db"
)

const sessionDuration = 30 * 24 * time.Hour

// Handler holds all auth dependencies.
type Handler struct {
	db       *db.DB
	botToken string
}

func NewHandler(database *db.DB, botToken string) *Handler {
	return &Handler{db: database, botToken: botToken}
}

// ── Telegram verification ────────────────────────────────────────────────────

type TelegramUser struct {
	ID           int64  `json:"id"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Username     string `json:"username"`
	PhotoURL     string `json:"photo_url"`
	AuthDate     int64  `json:"auth_date"`
	Hash         string `json:"hash"`
}

// VerifyTelegramAuth checks the HMAC-SHA256 signature produced by the Telegram
// Login Widget. See https://core.telegram.org/widgets/login#checking-authorization
func VerifyTelegramAuth(u *TelegramUser, botToken string) bool {
	// Build the data-check-string: alphabetically sorted "key=value" pairs,
	// one per line, excluding the "hash" field.
	fields := map[string]string{
		"id":         strconv.FormatInt(u.ID, 10),
		"first_name": u.FirstName,
		"auth_date":  strconv.FormatInt(u.AuthDate, 10),
	}
	if u.LastName != "" {
		fields["last_name"] = u.LastName
	}
	if u.Username != "" {
		fields["username"] = u.Username
	}
	if u.PhotoURL != "" {
		fields["photo_url"] = u.PhotoURL
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+fields[k])
	}
	dataCheckString := strings.Join(parts, "\n")

	// secret_key = SHA256(bot_token) — NOT HMAC
	h := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, h[:])
	mac.Write([]byte(dataCheckString))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(u.Hash))
}

// ── Session helpers ──────────────────────────────────────────────────────────

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ── Context key ─────────────────────────────────────────────────────────────

type ctxKey struct{}

func AccountFromCtx(ctx context.Context) *db.Account {
	a, _ := ctx.Value(ctxKey{}).(*db.Account)
	return a
}

// WithAccount returns a copy of ctx with the account stored under the package-private key.
// Use this in middlewares outside the auth package (e.g., gateway.RequireAPIKey).
func WithAccount(ctx context.Context, a *db.Account) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// ── Middleware ───────────────────────────────────────────────────────────────

// RequireSession extracts the Bearer token from Authorization header,
// looks it up in the sessions table, and injects the account into context.
func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeErr(w, http.StatusUnauthorized, "missing token")
			return
		}
		account, err := h.db.GetSessionAccount(r.Context(), token)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, account)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if strings.HasPrefix(v, "Bearer ") {
		return strings.TrimPrefix(v, "Bearer ")
	}
	return ""
}

// ── HTTP handlers ────────────────────────────────────────────────────────────

// HandleTelegramAuth verifies the Telegram login callback and issues a session.
func (h *Handler) HandleTelegramAuth(w http.ResponseWriter, r *http.Request) {
	var u TelegramUser
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if !VerifyTelegramAuth(&u, h.botToken) {
		writeErr(w, http.StatusUnauthorized, "telegram auth verification failed")
		return
	}

	// Auth date must be within the last hour to prevent replay attacks.
	if time.Since(time.Unix(u.AuthDate, 0)) > time.Hour {
		writeErr(w, http.StatusUnauthorized, "auth_date too old")
		return
	}

	username := u.Username
	if username == "" {
		username = fmt.Sprintf("user%d", u.ID)
	}

	account, err := h.db.UpsertAccount(r.Context(), u.ID, username)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "account error")
		return
	}

	token, err := generateToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "session error")
		return
	}

	expires := time.Now().Add(sessionDuration)
	if err := h.db.CreateSession(r.Context(), account.ID, token, expires); err != nil {
		writeErr(w, http.StatusInternalServerError, "session error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// HandleMe returns the authenticated account as a User object.
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	account := AccountFromCtx(r.Context())
	if account == nil {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, accountToUser(account))
}

// HandleLogout invalidates the session token.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token != "" {
		h.db.DeleteSession(r.Context(), token)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ── Response helpers ─────────────────────────────────────────────────────────

type userResponse struct {
	ID               int64  `json:"id"`
	TelegramID       int64  `json:"telegram_id"`
	TelegramUsername string `json:"telegram_username"`
	DID              string `json:"did"`
	Tier             string `json:"tier"`
	CreatedAt        string `json:"created_at"`
}

func accountToUser(a *db.Account) userResponse {
	return userResponse{
		ID:               a.ID,
		TelegramID:       a.TelegramID,
		TelegramUsername: a.TelegramUsername,
		DID:              a.DID,
		Tier:             a.Tier,
		CreatedAt:        a.CreatedAt.Format(time.RFC3339),
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
