package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/forumline/forumline/services/forumline-api/store"
	shared "github.com/forumline/forumline/shared-go"
)

type NotificationHandler struct {
	Store *store.Store
}

func NewNotificationHandler(s *store.Store) *NotificationHandler {
	return &NotificationHandler{Store: s}
}

type aggregatedNotification struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Link        string `json:"link"`
	Read        bool   `json:"read"`
	Timestamp   string `json:"timestamp"`
	ForumDomain string `json:"forum_domain"`
	ForumName   string `json:"forum_name"`
}

// HandleNotifications handles GET /api/notifications — aggregates from all user's forums.
func (h *NotificationHandler) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	memberships, err := h.Store.ListMemberships(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch memberships"})
		return
	}

	if len(memberships) == 0 {
		writeJSON(w, http.StatusOK, []aggregatedNotification{})
		return
	}

	token := signForumlineToken(userID)
	if token == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
		return
	}

	var mu sync.Mutex
	var items []aggregatedNotification
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	for _, m := range memberships {
		if m.NotificationsMuted {
			continue
		}
		wg.Add(1)
		go func(apiBase, forumName, forumDomain string) {
			defer wg.Done()
			notifs, err := fetchForumNotifications(ctx, apiBase, token)
			if err != nil {
				log.Printf("[notifications] failed to fetch from %s: %v", forumDomain, err)
				return
			}
			mu.Lock()
			for _, n := range notifs {
				n.ForumName = forumName
				items = append(items, n)
			}
			mu.Unlock()
		}(m.APIBase, m.ForumName, m.ForumDomain)
	}

	wg.Wait()

	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp > items[j].Timestamp
	})

	if len(items) > 50 {
		items = items[:50]
	}
	if items == nil {
		items = []aggregatedNotification{}
	}

	writeJSON(w, http.StatusOK, items)
}

// HandleMarkRead handles POST /api/notifications/read — proxies to the forum.
func (h *NotificationHandler) HandleMarkRead(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())

	var body struct {
		ID          string `json:"id"`
		ForumDomain string `json:"forum_domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID == "" || body.ForumDomain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id and forum_domain required"})
		return
	}

	memberships, err := h.Store.ListMemberships(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch memberships"})
		return
	}

	var apiBase string
	for _, m := range memberships {
		if m.ForumDomain == body.ForumDomain {
			apiBase = m.APIBase
			break
		}
	}
	if apiBase == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Forum not found in memberships"})
		return
	}

	token := signForumlineToken(userID)
	if token == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	payload, _ := json.Marshal(map[string]string{"id": body.ID})
	req, err := http.NewRequestWithContext(ctx, "POST", apiBase+"/notifications/read", bytes.NewReader(payload))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create request"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Forum unreachable"})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// HandleMarkAllRead handles POST /api/notifications/read-all — marks all read on all forums.
func (h *NotificationHandler) HandleMarkAllRead(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	memberships, err := h.Store.ListMemberships(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch memberships"})
		return
	}

	token := signForumlineToken(userID)
	if token == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
		return
	}

	// Fetch all notifications then mark each as read
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, m := range memberships {
		if m.NotificationsMuted {
			continue
		}
		wg.Add(1)
		go func(apiBase, domain string) {
			defer wg.Done()
			notifs, err := fetchForumNotifications(ctx, apiBase, token)
			if err != nil {
				return
			}
			for _, n := range notifs {
				if !n.Read {
					markForumNotificationRead(ctx, apiBase, token, n.ID)
				}
			}
		}(m.APIBase, m.ForumDomain)
	}
	wg.Wait()

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func fetchForumNotifications(ctx context.Context, apiBase, token string) ([]aggregatedNotification, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", apiBase+"/notifications", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var notifs []aggregatedNotification
	if err := json.NewDecoder(resp.Body).Decode(&notifs); err != nil {
		return nil, err
	}
	return notifs, nil
}

func markForumNotificationRead(ctx context.Context, apiBase, token, notifID string) {
	payload, _ := json.Marshal(map[string]string{"id": notifID})
	req, err := http.NewRequestWithContext(ctx, "POST", apiBase+"/notifications/read", bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

func signForumlineToken(userID string) string {
	secret := os.Getenv("FORUMLINE_JWT_SECRET")
	if secret == "" {
		return ""
	}
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
		Issuer:    "forumline-app",
	})
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		return ""
	}
	return tokenStr
}

