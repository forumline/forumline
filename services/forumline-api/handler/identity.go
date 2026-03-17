package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/forumline/forumline/backend/auth"
	"github.com/forumline/forumline/services/forumline-api/model"
	"github.com/forumline/forumline/services/forumline-api/service"
	"github.com/forumline/forumline/services/forumline-api/store"
)

type IdentityHandler struct {
	Store *store.Store
}

func NewIdentityHandler(s *store.Store) *IdentityHandler {
	return &IdentityHandler{Store: s}
}

func (h *IdentityHandler) HandleGetIdentity(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	p, err := h.Store.GetProfile(r.Context(), userID)
	if err != nil && p == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch profile"})
		return
	}

	// Auto-create profile on first login: fetch user info from Zitadel's userinfo endpoint
	if p == nil {
		p, err = h.provisionProfile(r.Context(), userID, r.Header.Get("Authorization"))
		if err != nil {
			log.Printf("[Identity] auto-provision failed for %s: %v", userID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create profile"})
			return
		}
		log.Printf("[Identity] auto-provisioned profile for user=%s username=%s", userID, strings.ReplaceAll(p.Username, "\n", ""))
	}

	avatarURL := ""
	if p.AvatarURL != nil {
		avatarURL = *p.AvatarURL
	}
	result := map[string]interface{}{
		"forumline_id": userID, "username": p.Username, "display_name": p.DisplayName,
		"avatar_url": avatarURL, "status_message": p.StatusMessage,
		"online_status": p.OnlineStatus, "show_online_status": p.ShowOnlineStatus,
	}
	if p.Bio != nil && *p.Bio != "" {
		result["bio"] = *p.Bio
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *IdentityHandler) HandleUpdateIdentity(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var body struct {
		Username         *string `json:"username"`
		StatusMessage    *string `json:"status_message"`
		OnlineStatus     *string `json:"online_status"`
		ShowOnlineStatus *bool   `json:"show_online_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	sets := make(map[string]interface{})

	if body.Username != nil {
		name := strings.TrimSpace(*body.Username)
		if name == "" || utf8.RuneCountInString(name) > 50 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Display name must be 1-50 characters"})
			return
		}
		sets["display_name"] = name
	}
	if body.StatusMessage != nil {
		msg := strings.TrimSpace(*body.StatusMessage)
		if utf8.RuneCountInString(msg) > 100 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Status message must be 100 characters or fewer"})
			return
		}
		sets["status_message"] = msg
	}
	if body.OnlineStatus != nil {
		switch *body.OnlineStatus {
		case "online", "away", "offline":
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "online_status must be online, away, or offline"})
			return
		}
		sets["online_status"] = *body.OnlineStatus
	}
	if body.ShowOnlineStatus != nil {
		sets["show_online_status"] = *body.ShowOnlineStatus
	}

	if len(sets) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if err := h.Store.UpdateProfile(r.Context(), userID, sets); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to update profile"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *IdentityHandler) HandleDeleteIdentity(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	// Delete from local DB first
	if err := h.Store.DeleteUser(r.Context(), userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to delete account"})
		return
	}

	// Delete from Zitadel (best-effort — profile is already gone locally)
	z, err := service.GetZitadelClient(r.Context())
	if err == nil {
		if err := z.DeleteUser(r.Context(), userID); err != nil {
			log.Printf("[Identity] warning: failed to delete Zitadel user %s: %v", userID, err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *IdentityHandler) HandleSearchProfiles(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "q parameter is required"})
		return
	}

	profiles, err := h.Store.SearchProfiles(r.Context(), q, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to search profiles"})
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

// zitadelUserinfoURL is the Zitadel OIDC userinfo endpoint URL.
var zitadelUserinfoURL string

func init() {
	if u := os.Getenv("ZITADEL_URL"); u != "" {
		zitadelUserinfoURL = u + "/oidc/v1/userinfo"
	}
}

// provisionProfile fetches user info from Zitadel's OIDC userinfo endpoint
// and creates a local profile.
func (h *IdentityHandler) provisionProfile(ctx context.Context, userID, authHeader string) (*model.Profile, error) {
	if zitadelUserinfoURL == "" {
		return nil, fmt.Errorf("ZITADEL_URL not set")
	}

	// Call Zitadel's OIDC userinfo endpoint with the user's own access token
	req, err := http.NewRequestWithContext(ctx, "GET", zitadelUserinfoURL, nil)
	if err != nil {
		return nil, err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo returned %d", resp.StatusCode)
	}

	var info struct {
		Sub               string `json:"sub"`
		PreferredUsername  string `json:"preferred_username"`
		Name              string `json:"name"`
		GivenName         string `json:"given_name"`
		FamilyName        string `json:"family_name"`
		Picture           string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}

	username := info.PreferredUsername
	if username == "" {
		username = "user_" + userID[len(userID)-6:]
	}
	displayName := info.Name
	if displayName == "" {
		displayName = strings.TrimSpace(info.GivenName + " " + info.FamilyName)
	}
	if displayName == "" {
		displayName = username
	}

	// Deduplicate username if it already exists (case-insensitive clash)
	if exists, _ := h.Store.UsernameExists(ctx, username); exists {
		username = username + "_" + userID[len(userID)-4:]
	}

	if err := h.Store.CreateProfile(ctx, userID, username, displayName, info.Picture); err != nil {
		return nil, fmt.Errorf("create profile: %w", err)
	}

	return &model.Profile{
		ID: userID, Username: username, DisplayName: displayName,
		StatusMessage: "", OnlineStatus: "online", ShowOnlineStatus: true,
	}, nil
}
