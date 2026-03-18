package pubsub

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	wmnats "github.com/ThreeDotsLabs/watermill-nats/v2/pkg/nats"
	"github.com/nats-io/nats.go"
)

// WatermillBus implements EventBus using Watermill's NATS adapter.
// It exposes the Watermill Subscriber separately for Router wiring in main.go.
type WatermillBus struct {
	pub    message.Publisher
	Sub    message.Subscriber
	logger watermill.LoggerAdapter
}

// NewWatermillBus connects to NATS and returns a Watermill-backed EventBus.
// The returned Bus.Sub field can be used with a Watermill Router for subscriptions.
func NewWatermillBus(url string) (*WatermillBus, error) {
	logger := watermill.NewStdLogger(false, false)

	natsOpts := []nats.Option{
		nats.Name("forumline"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Printf("NATS disconnected: %v", err)
			}
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Println("NATS reconnected")
		}),
	}

	// Core NATS (no JetStream) — fire-and-forget, same as our previous NATSBus.
	jsConfig := wmnats.JetStreamConfig{Disabled: true}

	marshaler := &wmnats.NATSMarshaler{}

	pub, err := wmnats.NewPublisher(wmnats.PublisherConfig{
		URL:         url,
		NatsOptions: natsOpts,
		Marshaler:   marshaler,
		JetStream:   jsConfig,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("watermill NATS publisher: %w", err)
	}

	sub, err := wmnats.NewSubscriber(wmnats.SubscriberConfig{
		URL:            url,
		CloseTimeout:   30 * time.Second,
		AckWaitTimeout: 30 * time.Second,
		NatsOptions:    natsOpts,
		Unmarshaler:    marshaler,
		JetStream:      jsConfig,
	}, logger)
	if err != nil {
		_ = pub.Close()
		return nil, fmt.Errorf("watermill NATS subscriber: %w", err)
	}

	log.Printf("NATS connected to %s (Watermill)", url)
	return &WatermillBus{pub: pub, Sub: sub, logger: logger}, nil
}

// Publish implements EventBus. Each message gets a unique correlation ID
// for end-to-end tracing through the Watermill Router handlers.
func (b *WatermillBus) Publish(_ context.Context, topic string, data []byte) error {
	msg := message.NewMessage(watermill.NewUUID(), data)
	middleware.SetCorrelationID(msg.UUID, msg)
	return b.pub.Publish(topic, msg)
}

// Close implements EventBus.
func (b *WatermillBus) Close() {
	_ = b.pub.Close()
	_ = b.Sub.Close()
}
