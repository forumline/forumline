package sse

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// Client represents a connected SSE client with a filter.
type Client struct {
	Channel    string                                 // event channel name
	Filter     map[string]string                      // e.g. {"recipient_id": "uuid"}
	FilterFunc func(data map[string]interface{}) bool // dynamic filter (used instead of Filter when set)
	Send       chan []byte
	Done       chan struct{}
}

// Hub manages SSE client subscriptions and fans out events to connected
// browsers. Events arrive via Feed() from an external source (NATS).
type Hub struct {
	mu      sync.RWMutex
	clients map[string][]*Client // channel -> clients
}

// NewHub creates an empty Hub. Call Feed() to push events in.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[string][]*Client),
	}
}

// broadcast sends a payload to all clients listening on the given channel,
// filtering by each client's filter criteria.
func (h *Hub) broadcast(channel string, payload []byte) {
	h.mu.RLock()
	clients := h.clients[channel]
	h.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	// Parse payload for filtering
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Printf("SSEHub: failed to parse payload: %v", err)
		return
	}

	for _, client := range clients {
		matched := false
		if client.FilterFunc != nil {
			matched = client.FilterFunc(data)
		} else {
			matched = matchesFilter(data, client.Filter)
		}
		if matched {
			select {
			case client.Send <- payload:
			default:
				// Client buffer full, skip
			}
		}
	}
}

// Register adds an SSE client to the hub.
func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	h.clients[client.Channel] = append(h.clients[client.Channel], client)
	h.mu.Unlock()
}

// Unregister removes an SSE client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	clients := h.clients[client.Channel]
	for i, c := range clients {
		if c == client {
			h.clients[client.Channel] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
}

// Serve writes SSE events to the response writer for the given client.
func Serve(w http.ResponseWriter, r *http.Request, client *Client) {
	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	if err := rc.Flush(); err != nil {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-client.Done:
			return
		case data := <-client.Send:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			_ = rc.Flush()
		}
	}
}

// MultiClient fans multiple channel subscriptions into a single HTTP response.
// Each event is tagged with the SSE "event:" field so clients can use addEventListener.
type MultiClient struct {
	Entries []MultiEntry
	Send    chan TaggedEvent
	Done    chan struct{}
}

// MultiEntry describes one channel subscription within a multi-client.
type MultiEntry struct {
	Channel    string
	EventType  string // SSE event: field (e.g. "dm", "notification", "call")
	FilterFunc func(data map[string]interface{}) bool
}

// TaggedEvent carries a payload with its event type for ServeMulti.
type TaggedEvent struct {
	EventType string
	Data      []byte
}

// RegisterMulti registers a MultiClient by creating internal per-channel
// clients that fan into the shared Send channel.
func (h *Hub) RegisterMulti(mc *MultiClient) []*Client {
	clients := make([]*Client, len(mc.Entries))
	for i, entry := range mc.Entries {
		client := &Client{
			Channel:    entry.Channel,
			FilterFunc: entry.FilterFunc,
			Send:       make(chan []byte, 32),
			Done:       mc.Done,
		}
		clients[i] = client
		h.Register(client)

		// Fan each internal client's Send into the multi-client's tagged Send
		go func(c *Client, eventType string) {
			for {
				select {
				case <-mc.Done:
					return
				case data, ok := <-c.Send:
					if !ok {
						return
					}
					select {
					case mc.Send <- TaggedEvent{EventType: eventType, Data: data}:
					case <-mc.Done:
						return
					}
				}
			}
		}(client, entry.EventType)
	}
	return clients
}

// UnregisterMulti removes all internal clients for a multi-client.
func (h *Hub) UnregisterMulti(clients []*Client) {
	for _, c := range clients {
		h.Unregister(c)
	}
}

// ServeMulti writes tagged SSE events to the response from a MultiClient.
func ServeMulti(w http.ResponseWriter, r *http.Request, mc *MultiClient) {
	rc := http.NewResponseController(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	if err := rc.Flush(); err != nil {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-mc.Done:
			return
		case tagged := <-mc.Send:
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", tagged.EventType, tagged.Data); err != nil {
				return
			}
			_ = rc.Flush()
		}
	}
}

// Feed injects an event into the hub from an external source (e.g. NATS).
// This decouples event production from the hub's client management, allowing
// any transport to feed events into the SSE fan-out.
func (h *Hub) Feed(channel string, payload []byte) {
	h.broadcast(channel, payload)
}

func matchesFilter(data map[string]interface{}, filter map[string]string) bool {
	for key, expected := range filter {
		val, ok := data[key]
		if !ok {
			return false
		}
		if fmt.Sprintf("%v", val) != expected {
			return false
		}
	}
	return true
}
