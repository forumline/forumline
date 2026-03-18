package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmmetrics "github.com/ThreeDotsLabs/watermill/components/metrics"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/forumline/forumline/backend/auth"
	"github.com/forumline/forumline/backend/db"
	"github.com/forumline/forumline/backend/httpkit"
	"github.com/forumline/forumline/backend/metrics"
	"github.com/forumline/forumline/backend/pubsub"
	"github.com/forumline/forumline/backend/sse"
	"github.com/forumline/forumline/backend/valkey"
	localdb "github.com/forumline/forumline/services/forumline-api/db"
	"github.com/forumline/forumline/services/forumline-api/realtime"
	"github.com/forumline/forumline/services/forumline-api/service"
	"github.com/forumline/forumline/services/forumline-api/store"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Auth (Zitadel JWT validation via JWKS)
	auth.MustInitAuth(ctx)

	// Database
	rawPool, err := db.NewPool(ctx)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer rawPool.Close()
	pool := db.NewObservablePool(rawPool)

	// Run pending migrations (goose, embedded SQL files)
	if err := db.RunMigrations(ctx, os.Getenv("DATABASE_URL"), localdb.Migrations); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Valkey (Redis-compatible cache) — nil if VALKEY_URL not set
	valkeyClient := valkey.NewClient(ctx)
	defer valkey.Close(valkeyClient)

	// SSE hub — fan-out engine for realtime events to browser SSE clients.
	sseHub := sse.NewHub()

	// Store + services (needed before push listener wiring)
	s := store.New(pool)

	// Validate schema on startup (warns about UUID/TEXT mismatches, missing FKs)
	s.ValidateSchema(ctx)

	// Clean up stale calls from previous server run (SSE drops on restart
	// leave calls stuck in ringing/active state, blocking new calls).
	if tag, err := pool.Exec(ctx,
		`UPDATE forumline_calls SET status = CASE WHEN status = 'ringing' THEN 'missed' ELSE 'completed' END, ended_at = now()
		 WHERE status IN ('ringing', 'active')`); err != nil {
		log.Printf("Warning: failed to clean up stale calls: %v", err)
	} else if tag.RowsAffected() > 0 {
		log.Printf("Cleaned up %d stale call(s) from previous run", tag.RowsAffected())
	}

	pushSvc := service.NewPushService(s)
	pushListener := realtime.NewPushListener(s, pushSvc)

	// Event bus: NATS via Watermill (required for all realtime events).
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		log.Fatal("NATS_URL is required — realtime events need NATS")
	}

	bus, err := pubsub.NewWatermillBus(natsURL)
	if err != nil {
		log.Fatalf("failed to connect to NATS: %v", err)
	}
	defer bus.Close()
	var eventBus pubsub.EventBus = bus

	// Watermill Router for subscriptions
	wmLogger := watermill.NewStdLogger(false, false)
	wmRouter, err := message.NewRouter(message.RouterConfig{}, wmLogger)
	if err != nil {
		log.Fatalf("failed to create Watermill router: %v", err)
	}
	wmRouter.AddMiddleware(middleware.Recoverer)
	wmRouter.AddMiddleware(middleware.CorrelationID)

	// Prometheus metrics for Watermill handlers (handler duration, message counts)
	wmMetrics := wmmetrics.NewPrometheusMetricsBuilder(prometheus.DefaultRegisterer, "forumline_api", "watermill")
	wmMetrics.AddPrometheusRouterMetrics(wmRouter)

	// SSE Hub subscriptions
	for _, ch := range []string{"dm_changes", "call_signal", "forumline_notification_changes"} {
		ch := ch
		wmRouter.AddConsumerHandler("sse-"+ch, ch, bus.Sub, func(msg *message.Message) error {
			sseHub.Feed(ch, msg.Payload)
			return nil
		})
	}

	// Push notification subscription (with retry — missed push = missed DM notification)
	retryMw := middleware.Retry{
		MaxRetries:      3,
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      2,
		Logger:          wmLogger,
	}
	pushHandler := wmRouter.AddConsumerHandler("push-dm", "push_dm", bus.Sub, func(msg *message.Message) error {
		return pushListener.HandlePayload(ctx, msg.Payload)
	})
	pushHandler.AddMiddleware(retryMw.Middleware)

	go func() {
		if err := wmRouter.Run(ctx); err != nil {
			log.Printf("Watermill router stopped: %v", err)
		}
	}()

	log.Println("realtime: Watermill event bus active")

	// Router
	router := newRouter(s, sseHub, valkeyClient, eventBus)
	router.Handle("GET /metrics", metrics.Handler())

	// Wrap with global middleware
	var handler http.Handler = router
	handler = metrics.Middleware("forumline_api")(handler)
	handler = httpkit.CORSMiddleware(handler)
	handler = httpkit.SecurityHeaders(handler)

	// Static file serving (SPA fallback)
	handler = spaHandler(handler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	// #nosec G706 -- port is from trusted env var
	log.Printf("forumline server listening on http://localhost:%s", port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// spaHandler serves static files from ./dist and falls back to index.html
// for navigation routes (paths without file extensions).
func spaHandler(apiHandler http.Handler) http.Handler {
	distDir := "./dist"
	fileServer := http.FileServer(http.Dir(distDir))

	// Build index.html with injected runtime config (ZITADEL_CLIENT_ID etc.)
	// so the frontend can read window.ZITADEL_CLIENT_ID without an extra fetch.
	indexHTML := buildIndexHTML(distDir)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API, Connect RPC, and metrics routes go to the router
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/forumline.") || r.URL.Path == "/metrics" {
			apiHandler.ServeHTTP(w, r)
			return
		}

		// Try to serve the static file
		path := filepath.Join(distDir, r.URL.Path)
		if info, err := os.Stat(path); err == nil && !info.IsDir() { // #nosec G703 -- path is cleaned by http.Dir
			// Hashed assets (Vite output) are immutable — cache forever
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// If path looks like a missing static asset, return 404.
		// Only check known asset extensions — domain-like paths (e.g.
		// /forum/forumline.net) must fall through to the SPA.
		if ext := filepath.Ext(r.URL.Path); isStaticAssetExt(ext) {
			http.NotFound(w, r)
			return
		}

		// SPA fallback — serve index.html with injected config
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
}

// buildIndexHTML reads dist/index.html and injects a <script> tag with
// runtime config (ZITADEL_CLIENT_ID, GLITCHTIP_DSN) right before </head>.
func buildIndexHTML(distDir string) []byte {
	raw, err := os.ReadFile(filepath.Join(distDir, "index.html"))
	if err != nil {
		log.Printf("Warning: could not read index.html for config injection: %v", err)
		return raw
	}

	var configs []string
	if v := os.Getenv("ZITADEL_CLIENT_ID"); v != "" {
		configs = append(configs, fmt.Sprintf("window.ZITADEL_CLIENT_ID=%q;", v))
	}
	if v := os.Getenv("GLITCHTIP_DSN"); v != "" {
		configs = append(configs, fmt.Sprintf("window.GLITCHTIP_DSN=%q;", v))
	}
	if len(configs) == 0 {
		return raw
	}

	configScript := "<script>" + strings.Join(configs, "") + "</script>"
	return bytes.Replace(raw, []byte("</head>"), []byte(configScript+"\n</head>"), 1)
}

// isStaticAssetExt returns true for file extensions that are expected static
// assets (JS, CSS, images, fonts, etc.). This avoids treating domain-like URL
// segments (e.g. "forumline.net") as missing files.
func isStaticAssetExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".js", ".mjs", ".css", ".html",
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".avif",
		".woff", ".woff2", ".ttf", ".eot",
		".json", ".map", ".webmanifest",
		".wasm", ".txt", ".xml":
		return true
	}
	return false
}
