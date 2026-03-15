package forum

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/forumline/forumline/services/hosted/forum/model"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	httphelper "github.com/zitadel/oidc/v3/pkg/http"
)

// oidcProvider is the Zitadel OIDC relying party (client), initialized lazily.
var oidcProvider rp.RelyingParty

// initOIDCProvider sets up the Zitadel OIDC relying party for this forum.
func (h *Handlers) initOIDCProvider() error {
	if oidcProvider != nil {
		return nil
	}
	if h.Config.ZitadelClientID == "" || h.Config.ZitadelURL == "" {
		return fmt.Errorf("ZITADEL_CLIENT_ID or ZITADEL_URL not configured")
	}

	redirectURI := h.Config.SiteURL + "/api/forumline/auth/callback"

	var err error
	// Use a 32-byte hash key for PKCE state cookies
	hashKey := []byte("forumline-oidc-pkce-hash-key-32b")
	cookieHandler := httphelper.NewCookieHandler(hashKey, nil)
	opts := []rp.Option{
		rp.WithPKCE(cookieHandler),
	}
	if h.Config.ZitadelClientSecret != "" {
		oidcProvider, err = rp.NewRelyingPartyOIDC(
			context.Background(),
			h.Config.ZitadelURL,
			h.Config.ZitadelClientID,
			h.Config.ZitadelClientSecret,
			redirectURI,
			[]string{"openid", "profile", "email"},
			opts...,
		)
	} else {
		// Public client (PKCE only, no secret)
		oidcProvider, err = rp.NewRelyingPartyOIDC(
			context.Background(),
			h.Config.ZitadelURL,
			h.Config.ZitadelClientID,
			"",
			redirectURI,
			[]string{"openid", "profile", "email"},
			opts...,
		)
	}
	return err
}

// HandleForumlineAuth handles GET /api/forumline/auth.
// Redirects to Zitadel's OIDC authorize endpoint.
func (h *Handlers) HandleForumlineAuth(w http.ResponseWriter, r *http.Request) {
	if err := h.initOIDCProvider(); err != nil {
		log.Printf("[Forumline:Auth] OIDC provider init failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}

	state := randomHex(16)

	// Store state in cookie for CSRF validation
	http.SetCookie(w, &http.Cookie{
		Name: "forumline_state", Value: state,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: true, MaxAge: 600,
	})

	// Build OIDC authorize URL with PKCE
	authURL := rp.AuthURL(state, oidcProvider)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleForumlineCallback handles GET /api/forumline/auth/callback.
// Exchanges the Zitadel auth code for tokens and creates/links local user.
func (h *Handlers) HandleForumlineCallback(w http.ResponseWriter, r *http.Request) {
	if err := h.initOIDCProvider(); err != nil {
		log.Printf("[Forumline:Callback] OIDC provider init failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}

	// Validate state
	cookies := parseCookies(r)
	state := r.URL.Query().Get("state")
	if state == "" || cookies["forumline_state"] != state {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "State mismatch — possible CSRF attack"})
		return
	}

	// Exchange code for tokens
	tokens, err := rp.CodeExchange[*oidc.IDTokenClaims](r.Context(), r.URL.Query().Get("code"), oidcProvider)
	if err != nil {
		log.Printf("[Forumline:Callback] Code exchange failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}

	// Extract identity from ID token claims
	claims := tokens.IDTokenClaims
	identity := &model.ForumlineIdentity{
		ForumlineID: claims.Subject,
		Username:    claims.PreferredUsername,
		DisplayName: claims.GivenName + " " + claims.FamilyName,
		AvatarURL:   claims.Picture,
	}
	if identity.Username == "" {
		identity.Username = string(claims.Email)
	}
	if identity.DisplayName == " " || identity.DisplayName == "" {
		identity.DisplayName = identity.Username
	}

	// Create or link local user
	localUserID, err := h.createOrLinkUser(r, identity)
	if err != nil {
		log.Printf("[Forumline:Callback] createOrLinkUser failed: %v", err)
		http.Redirect(w, r, h.Config.SiteURL+"/login?error=auth_failed", http.StatusFound)
		return
	}

	// Clear state cookie
	clearCookie(w, "forumline_state")

	// Set local session cookie with the Zitadel access token
	http.SetCookie(w, &http.Cookie{
		Name: "forumline_user_id", Value: localUserID,
		Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: true, MaxAge: 3600,
	})

	// Redirect with access token in hash (the Zitadel JWT access token is used for API calls)
	accessToken := tokens.AccessToken
	redirectURL := fmt.Sprintf("%s/#access_token=%s&type=bearer", h.Config.SiteURL, url.QueryEscape(accessToken))
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleForumlineToken handles GET /api/forumline/auth/forumline-token.
func (h *Handlers) HandleForumlineToken(w http.ResponseWriter, r *http.Request) {
	cookies := parseCookies(r)
	localUserID := cookies["forumline_user_id"]

	if localUserID != "" {
		forumlineID, err := h.Store.GetForumlineID(r.Context(), localUserID)
		if err != nil {
			log.Printf("query forumline_id error: %v", err)
		}
		if forumlineID == nil || *forumlineID == "" {
			writeJSON(w, http.StatusOK, map[string]interface{}{"forumline_access_token": nil})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"forumline_access_token": nil})
}

// HandleForumlineSession handles GET/DELETE /api/forumline/auth/session.
func (h *Handlers) HandleForumlineSession(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		h.handleDisconnect(w, r)
		return
	}

	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// Session is now managed by Zitadel — just return the user ID from cookie
	cookies := parseCookies(r)
	localUserID := cookies["forumline_user_id"]
	if localUserID == "" {
		writeJSON(w, http.StatusOK, nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"local_user_id": localUserID,
	})
}

// handleDisconnect clears Forumline session cookies.
func (h *Handlers) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, "forumline_user_id")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// createOrLinkUser creates or links a local user from a Forumline identity.
func (h *Handlers) createOrLinkUser(r *http.Request, identity *model.ForumlineIdentity) (string, error) {
	ctx := r.Context()

	existingID, err := h.Store.GetProfileIDByForumlineID(ctx, identity.ForumlineID)
	if err == nil && existingID != "" {
		_ = h.Store.UpdateDisplayNameAndAvatar(ctx, existingID, identity.DisplayName, identity.AvatarURL)
		return existingID, nil
	}

	if err := h.Store.CreateProfileHosted(ctx, identity); err != nil {
		return "", fmt.Errorf("create profile: %w", err)
	}
	return identity.ForumlineID, nil
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "",
		Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: true, MaxAge: -1,
	})
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
