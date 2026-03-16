package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/forumline/forumline/backend/auth"
	"github.com/forumline/forumline/services/forumline-api/store"
)

type ActivityHandler struct {
	Store *store.Store
}

func NewActivityHandler(s *store.Store) *ActivityHandler {
	return &ActivityHandler{Store: s}
}

// activityThread mirrors the forum server's thread JSON shape (subset).
type activityThread struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	PostCount  int             `json:"post_count"`
	LastPostAt *string         `json:"last_post_at"`
	CreatedAt  string          `json:"created_at"`
	Author     activityAuthor  `json:"author"`
	Category   activityCategory `json:"category"`
}

type activityAuthor struct {
	Username    string  `json:"username"`
	DisplayName *string `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

type activityCategory struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// activityItem is returned to the frontend.
type activityItem struct {
	ThreadID    string  `json:"thread_id"`
	ThreadTitle string  `json:"thread_title"`
	Author      string  `json:"author"`
	AvatarURL   *string `json:"avatar_url"`
	Action      string  `json:"action"` // "posted" or "active"
	ForumName   string  `json:"forum_name"`
	ForumDomain string  `json:"forum_domain"`
	Category    string  `json:"category"`
	Timestamp   string  `json:"timestamp"`
}

// HandleActivity handles GET /api/activity
func (h *ActivityHandler) HandleActivity(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	memberships, err := h.Store.ListMemberships(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch memberships"})
		return
	}

	if len(memberships) == 0 {
		writeJSON(w, http.StatusOK, []activityItem{})
		return
	}

	var mu sync.Mutex
	var items []activityItem
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	for _, m := range memberships {
		wg.Add(1)
		go func(webBase, forumName, forumDomain string) {
			defer wg.Done()

			threads, err := fetchForumThreads(ctx, webBase)
			if err != nil {
				log.Printf("[activity] failed to fetch threads from %s: %v", forumDomain, err)
				return
			}

			mu.Lock()
			for _, t := range threads {
				action := "posted"
				ts := t.CreatedAt
				if t.PostCount > 0 && t.LastPostAt != nil {
					action = "active"
					ts = *t.LastPostAt
				}

				author := t.Author.Username
				if t.Author.DisplayName != nil && *t.Author.DisplayName != "" {
					author = *t.Author.DisplayName
				}

				items = append(items, activityItem{
					ThreadID:    t.ID,
					ThreadTitle: t.Title,
					Author:      author,
					AvatarURL:   t.Author.AvatarURL,
					Action:      action,
					ForumName:   forumName,
					ForumDomain: forumDomain,
					Category:    t.Category.Name,
					Timestamp:   ts,
				})
			}
			mu.Unlock()
		}(m.WebBase, m.ForumName, m.ForumDomain)
	}

	wg.Wait()

	// Sort by timestamp descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp > items[j].Timestamp
	})

	// Limit to 20 items
	if len(items) > 20 {
		items = items[:20]
	}

	if items == nil {
		items = []activityItem{}
	}

	writeJSON(w, http.StatusOK, items)
}

func fetchForumThreads(ctx context.Context, webBase string) ([]activityThread, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", webBase+"/api/threads?limit=10", nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var threads []activityThread
	if err := json.NewDecoder(resp.Body).Decode(&threads); err != nil {
		return nil, err
	}
	return threads, nil
}
