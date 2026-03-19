package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/forumline/forumline/backend/pubsub"
)

type vapidConfig struct {
	PublicKey  string
	PrivateKey string
	Subject    string
}

type pushSubscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- env ---
	dbURL := requireEnv("DATABASE_URL")
	natsURL := requireEnv("NATS_URL")
	vapid := vapidConfig{
		PublicKey:  requireEnv("VAPID_PUBLIC_KEY"),
		PrivateKey: requireEnv("VAPID_PRIVATE_KEY"),
		Subject:    requireEnv("VAPID_EMAIL"),
	}
	if !strings.HasPrefix(vapid.Subject, "mailto:") {
		vapid.Subject = "mailto:" + vapid.Subject
	}

	// --- postgres ---
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("postgres connect: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("postgres ping: %v", err)
	}

	// --- nats ---
	bus, err := pubsub.NewWatermillBus(natsURL)
	if err != nil {
		log.Fatalf("nats connect: %v", err)
	}
	defer bus.Close()

	// --- watermill router ---
	wmLogger := watermill.NewStdLogger(false, false)
	router, err := message.NewRouter(message.RouterConfig{}, wmLogger)
	if err != nil {
		log.Fatalf("watermill router: %v", err)
	}
	router.AddMiddleware(middleware.Recoverer)
	router.AddMiddleware(middleware.CorrelationID)

	retryMw := middleware.Retry{
		MaxRetries:      3,
		InitialInterval: 500 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		Multiplier:      2,
		Logger:          wmLogger,
	}

	handler := router.AddConsumerHandler("push-dm", "push_dm", bus.Sub, func(msg *message.Message) error {
		return handlePushDm(ctx, pool, vapid, msg.Payload)
	})
	handler.AddMiddleware(retryMw.Middleware)

	// --- graceful shutdown ---
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		cancel()
	}()

	log.Println("push worker ready, subscribed to push_dm")
	if err := router.Run(ctx); err != nil {
		log.Printf("watermill router stopped: %v", err)
	}
}

func handlePushDm(ctx context.Context, pool *pgxpool.Pool, vapid vapidConfig, raw []byte) error {
	var payload struct {
		ConversationID string   `json:"conversation_id"`
		SenderID       string   `json:"sender_id"`
		MemberIDs      []string `json:"member_ids"`
		Content        string   `json:"content"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("parse push_dm payload: %w", err)
	}

	senderUsername := lookupUsername(ctx, pool, payload.SenderID)
	title := fmt.Sprintf("Message from %s", senderUsername)
	body := payload.Content
	if len(body) > 100 {
		body = body[:100]
	}

	for _, memberID := range payload.MemberIDs {
		if memberID == payload.SenderID {
			continue
		}
		sent := sendToUser(ctx, pool, vapid, memberID, title, body)
		if sent > 0 {
			log.Printf("[push] sent %d notifications for DM to %s", sent, memberID)
		}
	}
	return nil
}

func lookupUsername(ctx context.Context, pool *pgxpool.Pool, userID string) string {
	var username string
	err := pool.QueryRow(ctx,
		`SELECT username FROM forumline_profiles WHERE id = $1`, userID,
	).Scan(&username)
	if err != nil || username == "" {
		return "someone"
	}
	return username
}

func sendToUser(ctx context.Context, pool *pgxpool.Pool, vapid vapidConfig, userID, title, body string) int {
	rows, err := pool.Query(ctx,
		`SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1`, userID,
	)
	if err != nil {
		log.Printf("[push] query subscriptions for %s: %v", userID, err)
		return 0
	}
	defer rows.Close()

	var subs []pushSubscription
	for rows.Next() {
		var s pushSubscription
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			continue
		}
		subs = append(subs, s)
	}
	if len(subs) == 0 {
		return 0
	}

	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
	})

	var (
		sent           int32
		staleEndpoints []string
		mu             sync.Mutex
		wg             sync.WaitGroup
	)

	sem := make(chan struct{}, 10)
	for _, s := range subs {
		wg.Add(1)
		go func(endpoint, p256dh, auth string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			sub := &webpush.Subscription{
				Endpoint: endpoint,
				Keys:     webpush.Keys{P256dh: p256dh, Auth: auth},
			}

			resp, err := webpush.SendNotification(payload, sub, &webpush.Options{
				Subscriber:      vapid.Subject,
				VAPIDPublicKey:  vapid.PublicKey,
				VAPIDPrivateKey: vapid.PrivateKey,
			})
			if err != nil {
				log.Printf("[push] send error to %s: %v", endpoint[:min(len(endpoint), 60)], err)
				return
			}
			_ = resp.Body.Close()

			if resp.StatusCode == 410 || resp.StatusCode == 404 {
				mu.Lock()
				staleEndpoints = append(staleEndpoints, endpoint)
				mu.Unlock()
			} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				atomic.AddInt32(&sent, 1)
			} else {
				log.Printf("[push] unexpected status %d for %s", resp.StatusCode, endpoint[:min(len(endpoint), 60)])
			}
		}(s.Endpoint, s.P256dh, s.Auth)
	}
	wg.Wait()

	if len(staleEndpoints) > 0 {
		log.Printf("[push] cleaning up %d stale endpoints for %s", len(staleEndpoints), userID)
		_, _ = pool.Exec(ctx,
			`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = ANY($2)`,
			userID, staleEndpoints,
		)
	}

	return int(sent)
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}
