package forum

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	shared "github.com/forumline/forumline/shared-go"
)

// HandleNotifications handles GET /api/forumline/notifications.
func (h *Handlers) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	notifications, err := h.Store.ListForumlineNotifications(r.Context(), userID, 50, h.Config.Domain)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, notifications)
}

// HandleNotificationRead handles POST /api/forumline/notifications/read.
func (h *Handlers) HandleNotificationRead(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Notification ID required"})
		return
	}

	if err := h.Store.MarkNotificationRead(r.Context(), body.ID, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleUnread handles GET /api/forumline/unread.
func (h *Handlers) HandleUnread(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	notifCount, chatMentionCount, err := h.Store.CountUnread(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{
		"notifications": notifCount,
		"chat_mentions": chatMentionCount,
		"dms":           0,
	})
}

// HandleNotificationStream handles GET /api/forumline/notifications/stream (SSE).
func (h *Handlers) HandleNotificationStream(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	client := &shared.SSEClient{
		Channel: "notification_changes",
		Filter:  map[string]string{"user_id": userID},
		Send:    make(chan []byte, 32),
		Done:    make(chan struct{}),
	}

	h.SSEHub.Register(client)
	defer func() {
		h.SSEHub.Unregister(client)
		close(client.Done)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprint(w, ":connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ":heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case data := <-client.Send:
			var raw map[string]interface{}
			if err := json.Unmarshal(data, &raw); err == nil {
				event := map[string]interface{}{
					"id":           raw["id"],
					"type":         raw["type"],
					"title":        raw["title"],
					"body":         raw["message"],
					"timestamp":    raw["created_at"],
					"read":         raw["read"],
					"link":         raw["link"],
					"forum_domain": h.Config.Domain,
				}
				if event["link"] == nil {
					event["link"] = "/"
				}
				eventJSON, _ := json.Marshal(event)
				if _, err := fmt.Fprintf(w, "data: %s\n\n", eventJSON); err != nil {
					return
				}
			} else {
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					return
				}
			}
			flusher.Flush()
		}
	}
}
