package forumline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	shared "github.com/forumline/forumline/shared-go"
	webpush "github.com/SherClockHolmes/webpush-go"
	"golang.org/x/crypto/bcrypt"
)

// --- Memberships ---

func (h *Handlers) HandleGetMemberships(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx,
		`SELECT m.id, m.joined_at, m.forum_authed_at, m.notifications_muted,
		        f.domain, f.name, f.icon_url, f.api_base, f.web_base, f.capabilities
		 FROM forumline_memberships m
		 JOIN forumline_forums f ON f.id = m.forum_id
		 WHERE m.user_id = $1
		 ORDER BY m.joined_at DESC`, userID,
	)
	if err != nil {
		log.Printf("[Memberships] HandleGetMemberships query error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch memberships"})
		return
	}
	defer rows.Close()

	type membership struct {
		ForumDomain        string   `json:"forum_domain"`
		ForumName          string   `json:"forum_name"`
		ForumIconURL       *string  `json:"forum_icon_url"`
		APIBase            string   `json:"api_base"`
		WebBase            string   `json:"web_base"`
		Capabilities       []string `json:"capabilities"`
		JoinedAt           string   `json:"joined_at"`
		ForumAuthedAt      *string  `json:"forum_authed_at"`
		NotificationsMuted bool     `json:"notifications_muted"`
	}

	var memberships []membership
	for rows.Next() {
		var m membership
		var id string
		var joinedAt time.Time
		var forumAuthedAt *time.Time
		var notifMuted bool

		if err := rows.Scan(&id, &joinedAt, &forumAuthedAt, &notifMuted,
			&m.ForumDomain, &m.ForumName, &m.ForumIconURL, &m.APIBase, &m.WebBase, &m.Capabilities); err != nil {
			continue
		}
		m.JoinedAt = joinedAt.Format(time.RFC3339)
		if forumAuthedAt != nil {
			s := forumAuthedAt.Format(time.RFC3339)
			m.ForumAuthedAt = &s
		}
		m.NotificationsMuted = notifMuted
		memberships = append(memberships, m)
	}

	if memberships == nil {
		memberships = []membership{}
	}

	writeJSON(w, http.StatusOK, memberships)
}

func (h *Handlers) HandleUpdateMembershipAuth(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	var body struct {
		ForumDomain string `json:"forum_domain"`
		Authed      *bool  `json:"authed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.ForumDomain == "" || body.Authed == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing forum_domain or authed"})
		return
	}

	forumID := getForumIDByDomain(ctx, h.Pool.Pool, body.ForumDomain)
	if forumID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Forum not found"})
		return
	}

	if *body.Authed {
		_, err := h.Pool.Exec(ctx,
			`UPDATE forumline_memberships SET forum_authed_at = now()
			 WHERE user_id = $1 AND forum_id = $2`, userID, forumID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update auth state"})
			return
		}
	} else {
		_, err := h.Pool.Exec(ctx,
			`UPDATE forumline_memberships SET forum_authed_at = NULL
			 WHERE user_id = $1 AND forum_id = $2`, userID, forumID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update auth state"})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handlers) HandleToggleMembershipMute(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	var body struct {
		ForumDomain string `json:"forum_domain"`
		Muted       *bool  `json:"muted"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.ForumDomain == "" || body.Muted == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing forum_domain or muted"})
		return
	}

	forumID := getForumIDByDomain(ctx, h.Pool.Pool, body.ForumDomain)
	if forumID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Forum not found"})
		return
	}

	_, err := h.Pool.Exec(ctx,
		`UPDATE forumline_memberships SET notifications_muted = $1
		 WHERE user_id = $2 AND forum_id = $3`, *body.Muted, userID, forumID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update mute state"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handlers) HandleJoinForum(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	var body struct {
		ForumDomain string `json:"forum_domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.ForumDomain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing forum_domain"})
		return
	}

	// Look up forum by domain
	forumID := getForumIDByDomain(ctx, h.Pool.Pool, body.ForumDomain)

	// If not found, fetch manifest and auto-register
	if forumID == "" {
		manifest, err := fetchForumManifest(body.ForumDomain)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Forum not found and manifest fetch failed"})
			return
		}

		// Always use the requested domain, not the manifest's claim, to prevent spoofing
		manifest.Domain = body.ForumDomain

		manifestTags := normalizeTags(manifest.Tags)

		err = h.Pool.QueryRow(ctx,
			`INSERT INTO forumline_forums (domain, name, icon_url, api_base, web_base, capabilities, tags, approved)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, false)
			 ON CONFLICT (domain) DO UPDATE SET
			   name = EXCLUDED.name,
			   icon_url = EXCLUDED.icon_url,
			   api_base = EXCLUDED.api_base,
			   web_base = EXCLUDED.web_base,
			   capabilities = EXCLUDED.capabilities,
			   tags = EXCLUDED.tags
			 WHERE forumline_forums.approved = false
			 RETURNING id`,
			manifest.Domain, manifest.Name, manifest.IconURL,
			manifest.APIBase, manifest.WebBase, manifest.Capabilities, manifestTags,
		).Scan(&forumID)

		// If RETURNING returned nothing (forum exists and is approved), fetch the existing ID
		if err == pgx.ErrNoRows {
			forumID = getForumIDByDomain(ctx, h.Pool.Pool, body.ForumDomain)
			err = nil
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to register forum"})
			return
		}
	}

	// Create membership
	_, err := h.Pool.Exec(ctx,
		`INSERT INTO forumline_memberships (user_id, forum_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, forumID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to join forum"})
		return
	}

	// Fetch full forum details with joined_at
	var domain, name, apiBase, webBase string
	var iconURL *string
	var capabilities []string
	var joinedAt time.Time

	err = h.Pool.QueryRow(ctx,
		`SELECT f.domain, f.name, f.icon_url, f.api_base, f.web_base, f.capabilities, m.joined_at
		 FROM forumline_forums f
		 JOIN forumline_memberships m ON m.forum_id = f.id
		 WHERE f.id = $1 AND m.user_id = $2`, forumID, userID,
	).Scan(&domain, &name, &iconURL, &apiBase, &webBase, &capabilities, &joinedAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch forum details"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"domain":       domain,
		"name":         name,
		"icon_url":     iconURL,
		"api_base":     apiBase,
		"web_base":     webBase,
		"capabilities": capabilities,
		"joined_at":    joinedAt.Format(time.RFC3339),
	})
}

func (h *Handlers) HandleLeaveForum(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	var body struct {
		ForumDomain string `json:"forum_domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.ForumDomain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing forum_domain"})
		return
	}

	forumID := getForumIDByDomain(ctx, h.Pool.Pool, body.ForumDomain)
	if forumID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Forum not found"})
		return
	}

	_, err := h.Pool.Exec(ctx,
		`DELETE FROM forumline_memberships WHERE user_id = $1 AND forum_id = $2`,
		userID, forumID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to leave forum"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// --- Forums ---

func (h *Handlers) HandleListForums(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	// Search filter
	search := strings.TrimSpace(q.Get("q"))
	// Tag filter
	tag := strings.TrimSpace(q.Get("tag"))
	// Sort: popular (default), recent, name
	sort := q.Get("sort")
	if sort == "" {
		sort = "popular"
	}
	// Pagination
	limit := 50
	offset := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	// Build query dynamically
	query := `SELECT id, domain, name, icon_url, api_base, web_base, capabilities, description, screenshot_url, tags, member_count
		 FROM forumline_forums WHERE approved = true`
	var args []interface{}
	argIdx := 1

	if search != "" {
		// Escape ILIKE wildcards in user input
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search)
		query += fmt.Sprintf(` AND (name ILIKE $%d OR description ILIKE $%d OR domain ILIKE $%d)`, argIdx, argIdx, argIdx)
		args = append(args, "%"+escaped+"%")
		argIdx++
	}

	if tag != "" {
		query += fmt.Sprintf(` AND $%d = ANY(tags)`, argIdx)
		args = append(args, tag)
		argIdx++
	}

	switch sort {
	case "recent":
		query += ` ORDER BY created_at DESC`
	case "name":
		query += ` ORDER BY name`
	default: // "popular"
		query += ` ORDER BY member_count DESC, name`
	}

	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := h.Pool.Query(ctx, query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch forums"})
		return
	}
	defer rows.Close()

	var forums []map[string]interface{}
	for rows.Next() {
		var id, domain, name, apiBase, webBase string
		var iconURL, description, screenshotURL *string
		var capabilities, forumTags []string
		var memberCount int

		if err := rows.Scan(&id, &domain, &name, &iconURL, &apiBase, &webBase, &capabilities, &description, &screenshotURL, &forumTags, &memberCount); err != nil {
			continue
		}
		forum := map[string]interface{}{
			"id":             id,
			"domain":         domain,
			"name":           name,
			"icon_url":       iconURL,
			"api_base":       apiBase,
			"web_base":       webBase,
			"capabilities":   capabilities,
			"description":    description,
			"screenshot_url": screenshotURL,
			"tags":           forumTags,
			"member_count":   memberCount,
		}
		forums = append(forums, forum)
	}

	if forums == nil {
		forums = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, forums)
}

// HandleListForumTags returns all unique tags used by approved forums.
func (h *Handlers) HandleListForumTags(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx,
		`SELECT DISTINCT unnest(tags) AS tag FROM forumline_forums WHERE approved = true ORDER BY tag`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch tags"})
		return
	}
	defer rows.Close()

	var tagList []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			continue
		}
		tagList = append(tagList, tag)
	}

	if tagList == nil {
		tagList = []string{}
	}

	writeJSON(w, http.StatusOK, tagList)
}

// HandleRecommendedForums returns forums that the user's forum-mates have joined
// but the user hasn't. This is the "waggle dance" — peer signal amplification.
func (h *Handlers) HandleRecommendedForums(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	// Find forums that people who share forums with this user are also in,
	// ranked by how many shared-forum-mates are members.
	// Excludes forums the user is already in.
	rows, err := h.Pool.Query(ctx,
		`WITH my_forums AS (
			SELECT forum_id FROM forumline_memberships WHERE user_id = $1
		),
		forum_mates AS (
			SELECT DISTINCT m.user_id
			FROM forumline_memberships m
			JOIN my_forums mf ON m.forum_id = mf.forum_id
			WHERE m.user_id != $1
		)
		SELECT f.id, f.domain, f.name, f.icon_url, f.api_base, f.web_base,
		       f.capabilities, f.description, f.screenshot_url, f.tags, f.member_count,
		       COUNT(m2.user_id) AS shared_member_count
		FROM forumline_memberships m2
		JOIN forum_mates fm ON m2.user_id = fm.user_id
		JOIN forumline_forums f ON f.id = m2.forum_id
		WHERE f.approved = true
		  AND f.id NOT IN (SELECT forum_id FROM my_forums)
		GROUP BY f.id
		ORDER BY shared_member_count DESC, f.member_count DESC
		LIMIT 10`,
		userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch recommendations"})
		return
	}
	defer rows.Close()

	var forums []map[string]interface{}
	for rows.Next() {
		var id, domain, name, apiBase, webBase string
		var iconURL, description, screenshotURL *string
		var capabilities, forumTags []string
		var memberCount, sharedMemberCount int

		if err := rows.Scan(&id, &domain, &name, &iconURL, &apiBase, &webBase,
			&capabilities, &description, &screenshotURL, &forumTags, &memberCount,
			&sharedMemberCount); err != nil {
			continue
		}
		forum := map[string]interface{}{
			"id":                  id,
			"domain":              domain,
			"name":                name,
			"icon_url":            iconURL,
			"api_base":            apiBase,
			"web_base":            webBase,
			"capabilities":       capabilities,
			"description":        description,
			"screenshot_url":     screenshotURL,
			"tags":               forumTags,
			"member_count":       memberCount,
			"shared_member_count": sharedMemberCount,
		}
		forums = append(forums, forum)
	}

	if forums == nil {
		forums = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, forums)
}

func (h *Handlers) HandleRegisterForum(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	var body struct {
		Domain       string   `json:"domain"`
		Name         string   `json:"name"`
		APIBase      string   `json:"api_base"`
		WebBase      string   `json:"web_base"`
		Capabilities []string `json:"capabilities"`
		Description  *string  `json:"description"`
		Tags         []string `json:"tags"`
		RedirectURIs []string `json:"redirect_uris"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if body.Domain == "" || body.Name == "" || body.APIBase == "" || body.WebBase == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain, name, api_base, and web_base are required"})
		return
	}

	// Validate domain
	if err := validateDomain(body.Domain); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid domain: %v", err)})
		return
	}

	// Validate URLs
	for _, u := range []string{body.APIBase, body.WebBase} {
		if _, err := url.ParseRequestURI(u); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid URL: %s", u)})
			return
		}
	}

	// Check quota (max 5)
	var count int
	if err := h.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM forumline_forums WHERE owner_id = $1`, userID).Scan(&count); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to check forum quota"})
		return
	}
	if count >= 5 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Maximum of 5 forums per user"})
		return
	}

	// Check domain uniqueness
	var domainExists bool
	if err := h.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM forumline_forums WHERE domain = $1)`, body.Domain).Scan(&domainExists); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to check domain"})
		return
	}
	if domainExists {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "Forum with this domain is already registered"})
		return
	}

	// Create forum
	tags := normalizeTags(body.Tags)

	var forumID string
	err := h.Pool.QueryRow(ctx,
		`INSERT INTO forumline_forums (domain, name, api_base, web_base, capabilities, description, tags, owner_id, approved)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, false)
		 RETURNING id`,
		body.Domain, body.Name, body.APIBase, body.WebBase,
		body.Capabilities, body.Description, tags, userID,
	).Scan(&forumID)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to register forum"})
		return
	}

	// Generate OAuth client credentials
	clientIDBytes := make([]byte, 16)
	if _, err := rand.Read(clientIDBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate credentials"})
		return
	}
	clientID := hex.EncodeToString(clientIDBytes)

	clientSecretBytes := make([]byte, 32)
	if _, err := rand.Read(clientSecretBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate credentials"})
		return
	}
	clientSecret := hex.EncodeToString(clientSecretBytes)
	clientSecretHash, err := bcryptHash(clientSecret)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate credentials"})
		return
	}

	redirectURIs := body.RedirectURIs
	if len(redirectURIs) == 0 {
		redirectURIs = []string{body.WebBase + "/api/forumline/auth/callback"}
	}

	_, err = h.Pool.Exec(ctx,
		`INSERT INTO forumline_oauth_clients (forum_id, client_id, client_secret_hash, redirect_uris)
		 VALUES ($1, $2, $3, $4)`,
		forumID, clientID, clientSecretHash, redirectURIs,
	)
	if err != nil {
		shared.LogIfErr(ctx, "rollback forum after OAuth client creation failure", func() error {
			_, err := h.Pool.Exec(ctx, `DELETE FROM forumline_forums WHERE id = $1`, forumID)
			return err
		})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create OAuth credentials"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"forum_id":      forumID,
		"client_id":     clientID,
		"client_secret": clientSecret,
		"approved":      false,
		"message":       "Forum registered. OAuth credentials generated. Forum requires approval before appearing in public listings.",
	})
}

// --- Owner Forum Management ---

func (h *Handlers) HandleListOwnedForums(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx,
		`SELECT id, domain, name, icon_url, api_base, web_base, approved,
		        member_count, last_seen_at, consecutive_failures, created_at
		 FROM forumline_forums WHERE owner_id = $1
		 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch owned forums"})
		return
	}
	defer rows.Close()

	var forums []map[string]interface{}
	for rows.Next() {
		var id, domain, name, apiBase, webBase string
		var iconURL *string
		var approved bool
		var memberCount, consecutiveFailures int
		var lastSeenAt *time.Time
		var createdAt time.Time

		if err := rows.Scan(&id, &domain, &name, &iconURL, &apiBase, &webBase, &approved,
			&memberCount, &lastSeenAt, &consecutiveFailures, &createdAt); err != nil {
			continue
		}
		forum := map[string]interface{}{
			"id":                   id,
			"domain":               domain,
			"name":                 name,
			"icon_url":             iconURL,
			"api_base":             apiBase,
			"web_base":             webBase,
			"approved":             approved,
			"member_count":         memberCount,
			"consecutive_failures": consecutiveFailures,
			"created_at":           createdAt.Format(time.RFC3339),
		}
		if lastSeenAt != nil {
			forum["last_seen_at"] = lastSeenAt.Format(time.RFC3339)
		}
		forums = append(forums, forum)
	}

	if forums == nil {
		forums = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, forums)
}

func (h *Handlers) HandleDeleteForum(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	var body struct {
		ForumDomain string `json:"forum_domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.ForumDomain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing forum_domain"})
		return
	}

	forumID := getForumIDByDomain(ctx, h.Pool.Pool, body.ForumDomain)
	if forumID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Forum not found"})
		return
	}

	// Verify ownership
	var ownerID *string
	if err := h.Pool.QueryRow(ctx,
		`SELECT owner_id FROM forumline_forums WHERE id = $1`, forumID,
	).Scan(&ownerID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to verify ownership"})
		return
	}
	if ownerID == nil || *ownerID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "You are not the owner of this forum"})
		return
	}

	// Count members for response
	var memberCount int
	_ = h.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM forumline_memberships WHERE forum_id = $1`, forumID,
	).Scan(&memberCount)

	// Delete — cascades to memberships, OAuth clients, auth codes
	tag, err := h.Pool.Exec(ctx,
		`DELETE FROM forumline_forums WHERE id = $1 AND owner_id = $2`, forumID, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete forum"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Forum not found or not owned by you"})
		return
	}

	log.Printf("[Forums] Forum deleted: domain=%s id=%s owner=%s members_removed=%d", body.ForumDomain, forumID, userID, memberCount)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "members_removed": memberCount})
}

// --- Screenshot Update (service key auth) ---

func (h *Handlers) HandleUpdateScreenshot(w http.ResponseWriter, r *http.Request) {
	// Authenticate via service role key
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization"})
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	serviceKey := os.Getenv("FORUMLINE_SERVICE_ROLE_KEY")
	if serviceKey == "" || token != serviceKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authorization"})
		return
	}

	var body struct {
		Domain        string `json:"domain"`
		ScreenshotURL string `json:"screenshot_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Domain == "" || body.ScreenshotURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain and screenshot_url are required"})
		return
	}

	ctx := r.Context()
	tag, err := h.Pool.Exec(ctx,
		`UPDATE forumline_forums SET screenshot_url = $1, updated_at = now() WHERE domain = $2`,
		body.ScreenshotURL, body.Domain,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update screenshot"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "forum not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// HandleUpdateIcon updates a forum's icon_url (service key auth).
func (h *Handlers) HandleUpdateIcon(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization"})
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	serviceKey := os.Getenv("FORUMLINE_SERVICE_ROLE_KEY")
	if serviceKey == "" || token != serviceKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authorization"})
		return
	}

	var body struct {
		Domain  string `json:"domain"`
		IconURL string `json:"icon_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Domain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain is required"})
		return
	}

	ctx := r.Context()
	tag, err := h.Pool.Exec(ctx,
		`UPDATE forumline_forums SET icon_url = $1, updated_at = now() WHERE domain = $2`,
		body.IconURL, body.Domain,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update icon"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "forum not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// --- Forum Health (service key auth) ---

func (h *Handlers) HandleUpdateHealth(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization"})
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	serviceKey := os.Getenv("FORUMLINE_SERVICE_ROLE_KEY")
	if serviceKey == "" || token != serviceKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authorization"})
		return
	}

	var body struct {
		Domain  string `json:"domain"`
		Healthy bool   `json:"healthy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Domain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain is required"})
		return
	}

	ctx := r.Context()

	if body.Healthy {
		tag, err := h.Pool.Exec(ctx,
			`UPDATE forumline_forums SET last_seen_at = now(), consecutive_failures = 0
			 WHERE domain = $1`, body.Domain,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update health"})
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "forum not found"})
			return
		}

		// Re-approve if it was delisted due to failures
		_, _ = h.Pool.Exec(ctx,
			`UPDATE forumline_forums SET approved = true
			 WHERE domain = $1 AND approved = false AND consecutive_failures = 0
			 AND owner_id IS NOT NULL`, body.Domain,
		)

		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "action": "healthy"})
		return
	}

	// Unhealthy: increment failures and check thresholds
	var consecutiveFailures int
	var ownerID *string
	err := h.Pool.QueryRow(ctx,
		`UPDATE forumline_forums SET consecutive_failures = consecutive_failures + 1
		 WHERE domain = $1
		 RETURNING consecutive_failures, owner_id`, body.Domain,
	).Scan(&consecutiveFailures, &ownerID)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "forum not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update health"})
		return
	}

	action := "failure_recorded"

	// 3 consecutive failures: delist from public directory
	if consecutiveFailures >= 3 {
		tag, _ := h.Pool.Exec(ctx,
			`UPDATE forumline_forums SET approved = false WHERE domain = $1 AND approved = true`, body.Domain,
		)
		if tag.RowsAffected() > 0 {
			log.Printf("[Health] Forum delisted: domain=%s failures=%d", body.Domain, consecutiveFailures)
			action = "delisted"
		}
	}

	// 7 consecutive failures with no owner: auto-delete
	if consecutiveFailures >= 7 && ownerID == nil {
		tag, _ := h.Pool.Exec(ctx,
			`DELETE FROM forumline_forums WHERE domain = $1 AND owner_id IS NULL`, body.Domain,
		)
		if tag.RowsAffected() > 0 {
			log.Printf("[Health] Unowned forum auto-deleted: domain=%s failures=%d", body.Domain, consecutiveFailures)
			action = "auto_deleted"
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "action": action, "consecutive_failures": consecutiveFailures})
}

// HandleListAllForums returns all forums (regardless of approval status) for internal use.
func (h *Handlers) HandleListAllForums(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization"})
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	serviceKey := os.Getenv("FORUMLINE_SERVICE_ROLE_KEY")
	if serviceKey == "" || token != serviceKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authorization"})
		return
	}

	ctx := r.Context()
	rows, err := h.Pool.Query(ctx,
		`SELECT id, domain, name, icon_url, api_base, web_base, capabilities, approved, owner_id,
		        last_seen_at, consecutive_failures
		 FROM forumline_forums ORDER BY domain`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch forums"})
		return
	}
	defer rows.Close()

	var forums []map[string]interface{}
	for rows.Next() {
		var id, domain, name, apiBase, webBase string
		var iconURL, ownerID *string
		var capabilities []string
		var approved bool
		var lastSeenAt *time.Time
		var consecutiveFailures int

		if err := rows.Scan(&id, &domain, &name, &iconURL, &apiBase, &webBase,
			&capabilities, &approved, &ownerID, &lastSeenAt, &consecutiveFailures); err != nil {
			continue
		}
		forum := map[string]interface{}{
			"id":                   id,
			"domain":               domain,
			"name":                 name,
			"icon_url":             iconURL,
			"api_base":             apiBase,
			"web_base":             webBase,
			"capabilities":         capabilities,
			"approved":             approved,
			"has_owner":            ownerID != nil,
			"consecutive_failures": consecutiveFailures,
		}
		if lastSeenAt != nil {
			forum["last_seen_at"] = lastSeenAt.Format(time.RFC3339)
		}
		forums = append(forums, forum)
	}

	if forums == nil {
		forums = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, forums)
}

// --- Identity ---

func (h *Handlers) HandleGetIdentity(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	var username, displayName string
	var avatarURL, bio *string
	err := h.Pool.QueryRow(ctx,
		`SELECT username, display_name, avatar_url, bio FROM forumline_profiles WHERE id = $1`, userID,
	).Scan(&username, &displayName, &avatarURL, &bio)

	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Profile not found"})
		return
	}

	result := map[string]interface{}{
		"forumline_id": userID,
		"username":     username,
		"display_name": displayName,
		"avatar_url":   stringOrEmpty(avatarURL),
	}
	if bio != nil && *bio != "" {
		result["bio"] = *bio
	}

	writeJSON(w, http.StatusOK, result)
}

// --- Profile Search ---

func (h *Handlers) HandleSearchProfiles(w http.ResponseWriter, r *http.Request) {
	userID := shared.UserIDFromContext(r.Context())
	ctx := r.Context()

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "q parameter is required"})
		return
	}

	pattern := "%" + q + "%"
	rows, err := h.Pool.Query(ctx,
		`SELECT id, username, display_name, avatar_url
		 FROM forumline_profiles
		 WHERE id != $1 AND (username ILIKE $2 OR display_name ILIKE $2)
		 LIMIT 10`,
		userID, pattern,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to search profiles"})
		return
	}
	defer rows.Close()

	type profileResult struct {
		ID          string  `json:"id"`
		Username    string  `json:"username"`
		DisplayName *string `json:"display_name"`
		AvatarURL   *string `json:"avatar_url"`
	}

	var profiles []profileResult
	for rows.Next() {
		var p profileResult
		if err := rows.Scan(&p.ID, &p.Username, &p.DisplayName, &p.AvatarURL); err != nil {
			continue
		}
		profiles = append(profiles, p)
	}

	if profiles == nil {
		profiles = []profileResult{}
	}

	writeJSON(w, http.StatusOK, profiles)
}

// --- Push Notifications ---

func (h *Handlers) HandlePush(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")

	if action == "notify" && r.Method == http.MethodPost {
		h.handlePushNotify(w, r)
		return
	}

	if action == "subscribe" {
		h.handlePushSubscribe(w, r)
		return
	}

	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing or invalid action query param"})
}

func (h *Handlers) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	// Authenticate
	tokenStr := extractTokenFromRequest(r)
	if tokenStr == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization"})
		return
	}
	claims, err := shared.ValidateJWT(tokenStr)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	userID := claims.Subject
	ctx := r.Context()

	if r.Method == http.MethodPost {
		var body struct {
			Endpoint string `json:"endpoint"`
			Keys     struct {
				P256dh string `json:"p256dh"`
				Auth   string `json:"auth"`
			} `json:"keys"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if body.Endpoint == "" || body.Keys.P256dh == "" || body.Keys.Auth == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing subscription fields"})
			return
		}

		_, err := h.Pool.Exec(ctx,
			`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (user_id, endpoint) DO UPDATE SET p256dh = $3, auth = $4`,
			userID, body.Endpoint, body.Keys.P256dh, body.Keys.Auth,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	if r.Method == http.MethodDelete {
		var body struct {
			Endpoint string `json:"endpoint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Endpoint == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing endpoint"})
			return
		}
		shared.LogIfErr(ctx, "delete push subscription", func() error {
			_, err := h.Pool.Exec(ctx,
				`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`,
				userID, body.Endpoint,
			)
			return err
		})
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
}

func (h *Handlers) handlePushNotify(w http.ResponseWriter, r *http.Request) {
	// Auth: service key or OAuth client credentials
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Missing authorization"})
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	serviceKey := os.Getenv("FORUMLINE_SERVICE_ROLE_KEY")

	if token != serviceKey {
		// Check if it's an OAuth client secret hash
		ctx := r.Context()
		var clientExists bool
		if err := h.Pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM forumline_oauth_clients WHERE client_secret_hash = $1)`,
			token,
		).Scan(&clientExists); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to verify authorization"})
			return
		}

		if !clientExists {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Invalid authorization"})
			return
		}
	}

	var body struct {
		ForumlineID string `json:"forumline_id"`
		UserID      string `json:"user_id"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		Link        string `json:"link"`
		ForumDomain string `json:"forum_domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if body.Title == "" || body.Body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing title or body"})
		return
	}

	ctx := r.Context()
	targetUserID := body.UserID
	if targetUserID == "" && body.ForumlineID != "" {
		var id string
		err := h.Pool.QueryRow(ctx,
			`SELECT id FROM forumline_profiles WHERE id = $1`, body.ForumlineID,
		).Scan(&id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "User not found"})
			return
		}
		targetUserID = id
	}

	if targetUserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Missing user_id or forumline_id"})
		return
	}

	// Check if forum is muted
	if body.ForumDomain != "" {
		forumID := getForumIDByDomain(ctx, h.Pool.Pool, body.ForumDomain)
		if forumID != "" {
			var muted *bool
			if err := h.Pool.QueryRow(ctx,
				`SELECT notifications_muted FROM forumline_memberships
				 WHERE user_id = $1 AND forum_id = $2`, targetUserID, forumID,
			).Scan(&muted); err != nil && err != pgx.ErrNoRows {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to check mute status"})
				return
			}
			if muted != nil && *muted {
				writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "skipped": "forum_muted"})
				return
			}
		}
	}

	// Send push notifications
	sent := sendPushNotifications(ctx, h.Pool.Pool, targetUserID, body.Title, body.Body, body.Link, body.ForumDomain)

	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "sent": sent})
}

// sendPushNotifications sends web push to all subscriptions for a user.
func sendPushNotifications(ctx context.Context, pool *pgxpool.Pool, userID, title, body, link, forumDomain string) int {
	vapidPublicKey := os.Getenv("VAPID_PUBLIC_KEY")
	vapidPrivateKey := os.Getenv("VAPID_PRIVATE_KEY")
	vapidSubject := os.Getenv("VAPID_SUBJECT")

	if vapidPublicKey == "" || vapidPrivateKey == "" {
		return 0
	}

	rows, err := pool.Query(ctx,
		`SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1`, userID,
	)
	if err != nil {
		return 0
	}
	defer rows.Close()

	type sub struct {
		Endpoint string
		P256dh   string
		Auth     string
	}

	var subs []sub
	for rows.Next() {
		var s sub
		if err := rows.Scan(&s.Endpoint, &s.P256dh, &s.Auth); err != nil {
			continue
		}
		subs = append(subs, s)
	}

	if len(subs) == 0 {
		return 0
	}

	payload, _ := json.Marshal(map[string]string{
		"title":        title,
		"body":         body,
		"link":         link,
		"forum_domain": forumDomain,
	})

	var (
		sent           int32
		staleEndpoints []string
		mu             sync.Mutex
		wg             sync.WaitGroup
	)

	// Send push notifications concurrently (max 10 in parallel)
	sem := make(chan struct{}, 10)
	for _, s := range subs {
		wg.Add(1)
		go func(s sub) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			subscription := &webpush.Subscription{
				Endpoint: s.Endpoint,
				Keys: webpush.Keys{
					P256dh: s.P256dh,
					Auth:   s.Auth,
				},
			}

			resp, err := webpush.SendNotification(payload, subscription, &webpush.Options{
				Subscriber:      vapidSubject,
				VAPIDPublicKey:  vapidPublicKey,
				VAPIDPrivateKey: vapidPrivateKey,
			})
			if err != nil {
				return
			}
			_ = resp.Body.Close()

			if resp.StatusCode == 410 || resp.StatusCode == 404 {
				mu.Lock()
				staleEndpoints = append(staleEndpoints, s.Endpoint)
				mu.Unlock()
			} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				atomic.AddInt32(&sent, 1)
			}
		}(s)
	}
	wg.Wait()

	// Batch cleanup stale endpoints
	if len(staleEndpoints) > 0 {
		shared.LogIfErr(ctx, "cleanup stale push endpoints", func() error {
			_, err := pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = ANY($2)`, userID, staleEndpoints)
			return err
		})
	}

	return int(sent)
}

// --- Helpers ---

type forumManifest struct {
	ForumlineVersion string   `json:"forumline_version"`
	Name             string   `json:"name"`
	Domain           string   `json:"domain"`
	IconURL          string   `json:"icon_url"`
	APIBase          string   `json:"api_base"`
	WebBase          string   `json:"web_base"`
	Capabilities     []string `json:"capabilities"`
	Tags             []string `json:"tags"`
}

// validateDomain checks that a domain is a plausible public hostname,
// rejecting path separators, query strings, and private/loopback IPs.
func validateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain is empty")
	}
	// Reject characters that could be used for path traversal or injection
	if strings.ContainsAny(domain, "/#?@ \t\n\r") {
		return fmt.Errorf("domain contains invalid characters")
	}
	// Strip optional port for IP check
	host := domain
	if h, _, err := net.SplitHostPort(domain); err == nil {
		host = h
	}
	// Reject IP addresses (only allow hostnames)
	if ip := net.ParseIP(host); ip != nil {
		return fmt.Errorf("domain must be a hostname, not an IP address")
	}
	// Must contain at least one dot (e.g. "example.com")
	if !strings.Contains(host, ".") {
		return fmt.Errorf("domain must be a fully qualified hostname")
	}
	return nil
}

func fetchForumManifest(domain string) (*forumManifest, error) {
	if err := validateDomain(domain); err != nil {
		return nil, fmt.Errorf("invalid domain: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	manifestReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("https://%s/.well-known/forumline-manifest.json", domain), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create manifest request: %w", err)
	}
	resp, err := client.Do(manifestReq)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest returned status %d", resp.StatusCode)
	}

	var manifest forumManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	if manifest.Name == "" || manifest.APIBase == "" || manifest.WebBase == "" {
		return nil, fmt.Errorf("manifest missing required fields")
	}

	// Always use the requested domain — never trust the manifest's domain claim
	manifest.Domain = domain

	return &manifest, nil
}

// normalizeTags lowercases, trims, deduplicates, and caps tags.
// Max 10 tags, max 32 chars each.
func normalizeTags(raw []string) []string {
	if len(raw) == 0 {
		return []string{}
	}
	seen := make(map[string]bool)
	var result []string
	for _, t := range raw {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		// Truncate to 32 characters (rune-safe)
		if utf8.RuneCountInString(t) > 32 {
			runes := []rune(t)
			t = string(runes[:32])
		}
		seen[t] = true
		result = append(result, t)
		if len(result) >= 10 {
			break
		}
	}
	if result == nil {
		return []string{}
	}
	return result
}

func getForumIDByDomain(ctx context.Context, pool *pgxpool.Pool, domain string) string {
	var id string
	_ = pool.QueryRow(ctx, `SELECT id FROM forumline_forums WHERE domain = $1`, domain).Scan(&id)
	return id
}

type profileInfo struct {
	Username    string
	DisplayName string
	AvatarURL   *string
}

func fetchProfilesByIDs(ctx context.Context, pool *pgxpool.Pool, ids []string) map[string]*profileInfo {
	profiles := make(map[string]*profileInfo)
	if len(ids) == 0 {
		return profiles
	}

	rows, err := pool.Query(ctx,
		`SELECT id, username, display_name, avatar_url FROM forumline_profiles WHERE id = ANY($1)`, ids,
	)
	if err != nil {
		return profiles
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		p := &profileInfo{}
		if err := rows.Scan(&id, &p.Username, &p.DisplayName, &p.AvatarURL); err != nil {
			continue
		}
		profiles[id] = p
	}

	return profiles
}

func stringOrEmpty(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func trimString(s string) string {
	return strings.TrimSpace(s)
}

func bcryptHash(secret string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
