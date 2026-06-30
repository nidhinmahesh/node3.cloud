package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"node3.cloud/platform/internal/auth"
	"node3.cloud/platform/internal/billing"
	"node3.cloud/platform/internal/contracts"
	"node3.cloud/platform/internal/db"
	"node3.cloud/platform/internal/gateway"
	"node3.cloud/platform/internal/webhook"
)

func main() {
	cfg := loadConfig()

	// sigCtx is cancelled when SIGINT/SIGTERM is received; background workers
	// use it so they stop cleanly before the HTTP server drains.
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.NewWithRubixPool(sigCtx, cfg.databaseURL, cfg.rubixDatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()

	authH := auth.NewHandler(database, cfg.telegramBotToken)
	gwH := gateway.NewHandler(database, cfg.serverSecret)
	hookH := webhook.NewHandler(database)
	contractH := contracts.NewHandler(database, cfg.wasmDir, cfg.platformURL, cfg.callbackSecret)
	billingH := billing.NewHandler(database, gwH,
		cfg.lemonSecret, cfg.lemonAPIKey, cfg.lemonVariantID, cfg.platformURL)

	// Start background workers; they stop when sigCtx is cancelled.
	worker := webhook.NewWorker(database, 5*time.Second)
	go worker.Start(sigCtx)

	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			select {
			case <-sigCtx.Done():
				return
			case <-t.C:
				database.PurgeExpiredSessions(context.Background())
			}
		}
	}()

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(cfg.platformURL))

	// ── Public auth endpoints ────────────────────────────────────────────────
	r.Post("/api/auth/telegram", authH.HandleTelegramAuth)

	// ── Internal: node → platform callback (no session required) ────────────
	r.Post("/internal/sc/callback", contractH.HandleSCCallback)

	// ── Billing webhook (Lemon Squeezy → platform) ───────────────────────────
	r.Post("/api/lemon/webhook", billingH.HandleLemonWebhook)

	// ── Session-authenticated routes ─────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(authH.RequireSession)

		// Auth
		r.Get("/api/auth/me", authH.HandleMe)
		r.Post("/api/auth/logout", authH.HandleLogout)

		// API keys
		r.Get("/api/keys", gwH.HandleListKeys)
		r.Post("/api/keys", gwH.HandleCreateKey)
		r.Delete("/api/keys/{id}", gwH.HandleRevokeKey)

		// Usage
		r.Get("/api/usage", gwH.HandleGetUsage)

		// DID
		r.Post("/api/dids/create", gwH.HandleCreateDID)

		// Transaction relay (non-custodial signing)
		r.Post("/api/tx/initiate", gwH.HandleTxInitiate)
		r.Post("/api/tx/sign", gwH.HandleTxSign)

		// Webhooks
		r.Get("/api/webhooks", hookH.HandleList)
		r.Post("/api/webhooks", hookH.HandleCreate)
		r.Delete("/api/webhooks/{id}", hookH.HandleDelete)
		r.Get("/api/webhooks/{id}/deliveries", hookH.HandleDeliveries)

		// Hosted contracts
		r.Get("/api/contracts", contractH.HandleList)
		r.Post("/api/contracts/deploy", contractH.HandleDeploy)
		r.Get("/api/contracts/{id}/executions", contractH.HandleExecutions)

		// Billing
		r.Get("/api/billing", billingH.HandleGetBilling)
		r.Post("/api/billing/checkout", billingH.HandleCheckout)
		r.Post("/api/billing/cancel", billingH.HandleCancel)
	})

	// ── API-key-authenticated developer gateway ──────────────────────────────
	// All rubixgoplatform REST routes are forwarded verbatim to the user's
	// assigned node after key validation and quota enforcement.
	// The platform's own /api/dids/create and /api/contracts/deploy are the
	// managed paths; developers can also reach the raw node API here for
	// everything else (balance queries, transfers, FT/NFT operations, etc.).
	r.Group(func(r chi.Router) {
		r.Use(gwH.RequireAPIKey)
		r.HandleFunc("/rubix/*", gwH.HandleNodeProxy)
	})

	addr := fmt.Sprintf(":%s", cfg.port)
	log.Printf("node3.cloud platform listening on %s", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// 310s: covers /rubix/ proxy (nginx 300s) + generate(60s) + registerCallback(15s).
		// httputil.ReverseProxy streams the response, so WriteTimeout starts from first
		// byte written — in practice the node sends headers immediately and streams the
		// body, so the effective deadline is well within 310s for almost all operations.
		WriteTimeout: 310 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-sigCtx.Done()
	stop() // release signal resources immediately

	log.Println("shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("server stopped")
}

// ── Config ───────────────────────────────────────────────────────────────────

type config struct {
	databaseURL      string
	rubixDatabaseURL string
	telegramBotToken string
	lemonSecret      string
	lemonAPIKey      string
	lemonVariantID   string
	platformURL      string
	wasmDir          string
	serverSecret     string // HMAC key for DID password derivation — never rotated
	callbackSecret   string // HMAC key for per-contract callback URL tokens
	port             string
}

func loadConfig() config {
	databaseURL := requireEnv("DATABASE_URL")
	return config{
		databaseURL:      databaseURL,
		rubixDatabaseURL: getEnv("RUBIX_DATABASE_URL", databaseURL),
		telegramBotToken: requireEnv("TELEGRAM_BOT_TOKEN"),
		lemonSecret:      requireEnv("LEMON_WEBHOOK_SECRET"),
		lemonAPIKey:      requireEnv("LEMON_API_KEY"),
		lemonVariantID:   requireEnv("LEMON_VARIANT_ID"),
		platformURL:      getEnv("PLATFORM_URL", "https://node3.cloud"),
		wasmDir:          getEnv("WASM_DIR", "/opt/node3/wasm"),
		serverSecret:     requireEnv("SERVER_SECRET"),
		callbackSecret:   requireEnv("CALLBACK_SECRET"),
		port:             getEnv("PORT", "8080"),
	}
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ── CORS ─────────────────────────────────────────────────────────────────────

func corsMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == allowedOrigin || origin == "http://localhost:5173" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-API-Key")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
