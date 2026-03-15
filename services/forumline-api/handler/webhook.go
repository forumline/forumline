package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/forumline/forumline/services/forumline-api/store"
)

type WebhookHandler struct {
	Store *store.Store
}

func NewWebhookHandler(s *store.Store) *WebhookHandler {
	return &WebhookHandler{Store: s}
}

// HandleNotification handles POST /api/webhooks/notification.
// Forums call this to push notifications to forumline when they are created.
// Auth: forum_domain in the request body — the forum is identified by its registered domain.
func (h *WebhookHandler) HandleNotification(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ForumDomain     string `json:"forum_domain"`
		ForumlineUserID string `json:"forumline_user_id"`
		Type            string `json:"type"`
		Title           string `json:"title"`
		Body            string `json:"body"`
		Link            string `json:"link"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ForumDomain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "forum_domain is required"})
		return
	}
	if body.ForumlineUserID == "" || body.Type == "" || body.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "forumline_user_id, type, and title are required"})
		return
	}

	// Look up forum to get its name
	forumName := body.ForumDomain
	if name, err := h.Store.GetForumNameByDomain(r.Context(), body.ForumDomain); err == nil {
		forumName = name
	}

	link := body.Link
	if link == "" {
		link = "/"
	}

	if err := h.Store.InsertNotification(r.Context(), body.ForumlineUserID, body.ForumDomain, forumName, body.Type, body.Title, body.Body, link); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create notification"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

// HandleNotificationBatch handles POST /api/webhooks/notifications (batch).
func (h *WebhookHandler) HandleNotificationBatch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ForumDomain string `json:"forum_domain"`
		ForumName   string `json:"forum_name"`
		Items       []struct {
			ForumlineUserID string `json:"forumline_user_id"`
			Type            string `json:"type"`
			Title           string `json:"title"`
			Body            string `json:"body"`
			Link            string `json:"link"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	if body.ForumDomain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "forum_domain required"})
		return
	}

	forumDomain := body.ForumDomain
	forumName := body.ForumName
	if forumName == "" {
		if name, err := h.Store.GetForumNameByDomain(r.Context(), forumDomain); err == nil {
			forumName = name
		} else {
			forumName = forumDomain
		}
	}

	ctx := r.Context()
	inserted := 0
	for _, item := range body.Items {
		if item.ForumlineUserID == "" || item.Type == "" || item.Title == "" {
			continue
		}
		link := item.Link
		if link == "" {
			link = "/"
		}
		if err := h.Store.InsertNotification(ctx, item.ForumlineUserID, forumDomain, forumName, item.Type, item.Title, item.Body, link); err != nil {
			log.Printf("[webhook] batch insert error: %v", err)
			continue
		}
		inserted++
	}

	writeJSON(w, http.StatusCreated, map[string]int{"inserted": inserted})
}
