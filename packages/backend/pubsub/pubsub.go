// Package pubsub provides a transport-agnostic event bus for realtime events.
//
// The EventBus interface decouples event producers (service layer code) from
// the transport layer (NATS via Watermill). Consumers (SSE hub, push listener)
// subscribe via Watermill Router in main.go — not through this interface.
package pubsub

import "context"

// EventBus is the publish-side interface for realtime events.
// Services call Publish to send events; subscriptions are wired
// separately via Watermill Router in each service's main.go.
type EventBus interface {
	// Publish sends data on the given topic (e.g. "dm_changes").
	Publish(ctx context.Context, topic string, data []byte) error

	// Close tears down the connection. Safe to call multiple times.
	Close()
}
