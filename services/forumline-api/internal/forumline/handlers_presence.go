package forumline

import (
	"net/http"
	"strings"
	"sync"
	"time"

	shared "github.com/forumline/forumline/shared-go"
)

// PresenceTracker tracks which users are online via heartbeats.
// Users are considered online if they've sent a heartbeat within the TTL.
type PresenceTracker struct {
	mu       sync.RWMutex
	lastSeen map[string]time.Time // userID -> last heartbeat time
	ttl      time.Duration
}

// NewPresenceTracker creates a presence tracker with the given TTL.
func NewPresenceTracker(ttl time.Duration) *PresenceTracker {
	pt := &PresenceTracker{
		lastSeen: make(map[string]time.Time),
		ttl:      ttl,
	}
	// Periodic cleanup of stale entries
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			pt.cleanup()
		}
	}()
	return pt
}

func (pt *PresenceTracker) Touch(userID string) {
	pt.mu.Lock()
	pt.lastSeen[userID] = time.Now()
	pt.mu.Unlock()
}

func (pt *PresenceTracker) IsOnline(userID string) bool {
	pt.mu.RLock()
	t, ok := pt.lastSeen[userID]
	pt.mu.RUnlock()
	return ok && time.Since(t) < pt.ttl
}

func (pt *PresenceTracker) OnlineStatusBatch(userIDs []string) map[string]bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	now := time.Now()
	result := make(map[string]bool, len(userIDs))
	for _, id := range userIDs {
		t, ok := pt.lastSeen[id]
		result[id] = ok && now.Sub(t) < pt.ttl
	}
	return result
}

func (pt *PresenceTracker) cleanup() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	now := time.Now()
	for id, t := range pt.lastSeen {
		if now.Sub(t) >= pt.ttl {
			delete(pt.lastSeen, id)
		}
	}
}

// HandlePresenceHeartbeat records a heartbeat for the authenticated user.
func (h *Handlers) HandlePresenceHeartbeat(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	h.Presence.Touch(userID)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// HandlePresenceStatus returns online status for a list of user IDs.
func (h *Handlers) HandlePresenceStatus(w http.ResponseWriter, r *http.Request) {
	idsParam := r.URL.Query().Get("userIds")
	if idsParam == "" {
		writeJSON(w, http.StatusOK, map[string]bool{})
		return
	}

	userIDs := strings.Split(idsParam, ",")
	if len(userIDs) > 200 {
		userIDs = userIDs[:200]
	}

	status := h.Presence.OnlineStatusBatch(userIDs)
	writeJSON(w, http.StatusOK, status)
}
