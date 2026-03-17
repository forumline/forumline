package forum

import (
	"context"
	"net/http"

	"github.com/forumline/forumline/backend/auth"
	"github.com/forumline/forumline/forum/oapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// iconURLPtr returns a *string pointer if s is non-empty, nil otherwise.
func iconURLPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GetConfig serves /api/config for frontend discovery of forum name and mode.
func (h *Handlers) GetConfig(ctx context.Context, _ oapi.GetConfigRequestObject) (oapi.GetConfigResponseObject, error) {
	name := h.Config.ForumName
	if name == "" {
		name = h.Config.Domain
	}
	resp := oapi.ForumConfig{
		Name:       name,
		HostedMode: h.Config.HostedMode,
	}
	if h.Config.IconURL != "" {
		resp.IconUrl = &h.Config.IconURL
	}
	if h.Config.LiveKit != nil {
		resp.LivekitUrl = &h.Config.LiveKit.URL
	}
	return oapi.GetConfig200JSONResponse(resp), nil
}

// GetManifest serves /.well-known/forumline-manifest.json for forum discovery.
func (h *Handlers) GetManifest(ctx context.Context, _ oapi.GetManifestRequestObject) (oapi.GetManifestResponseObject, error) {
	name := h.Config.ForumName
	if name == "" {
		name = h.Config.Domain
	}
	return oapi.GetManifest200JSONResponse(oapi.ForumlineManifest{
		ForumlineVersion: "1",
		Name:             name,
		Domain:           h.Config.Domain,
		IconUrl:          iconURLPtr(h.Config.IconURL),
		ApiBase:          h.Config.SiteURL + "/api/forumline",
		WebBase:          h.Config.SiteURL,
		Capabilities: []oapi.ForumlineManifestCapabilities{
			oapi.Threads,
			oapi.Chat,
			oapi.Voice,
			oapi.Notifications,
		},
	}), nil
}

// ── Channel follows ──────────────────────────────────────────────────

// ListChannelFollows handles GET /api/channel-follows.
func (h *Handlers) ListChannelFollows(ctx context.Context, _ oapi.ListChannelFollowsRequestObject) (oapi.ListChannelFollowsResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)
	ids, err := h.Store.ListChannelFollows(ctx, userID)
	if err != nil {
		return oapi.ListChannelFollows500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	// Convert []string to []openapi_types.UUID
	uuids := make([]openapi_types.UUID, 0, len(ids))
	for _, id := range ids {
		var u openapi_types.UUID
		if err := u.UnmarshalText([]byte(id)); err == nil {
			uuids = append(uuids, u)
		}
	}
	return oapi.ListChannelFollows200JSONResponse(uuids), nil
}

// FollowChannel handles POST /api/channel-follows.
func (h *Handlers) FollowChannel(ctx context.Context, request oapi.FollowChannelRequestObject) (oapi.FollowChannelResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)

	if err := h.Store.AddChannelFollow(ctx, userID, request.Body.CategoryId.String()); err != nil {
		return oapi.FollowChannel500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	t := true
	return oapi.FollowChannel200JSONResponse{Ok: &t}, nil
}

// UnfollowChannel handles DELETE /api/channel-follows.
func (h *Handlers) UnfollowChannel(ctx context.Context, request oapi.UnfollowChannelRequestObject) (oapi.UnfollowChannelResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)

	if err := h.Store.RemoveChannelFollow(ctx, userID, request.Body.CategoryId.String()); err != nil {
		return oapi.UnfollowChannel500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	t := true
	return oapi.UnfollowChannel200JSONResponse{Ok: &t}, nil
}

// ── Notification preferences ─────────────────────────────────────────

// ListNotificationPreferences handles GET /api/notification-preferences.
func (h *Handlers) ListNotificationPreferences(ctx context.Context, _ oapi.ListNotificationPreferencesRequestObject) (oapi.ListNotificationPreferencesResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)
	prefs, err := h.Store.ListNotificationPrefs(ctx, userID)
	if err != nil {
		return oapi.ListNotificationPreferences500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListNotificationPreferences200JSONResponse(prefs), nil
}

// UpdateNotificationPreference handles PUT /api/notification-preferences.
func (h *Handlers) UpdateNotificationPreference(ctx context.Context, request oapi.UpdateNotificationPreferenceRequestObject) (oapi.UpdateNotificationPreferenceResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)
	body := request.Body

	if err := h.Store.UpsertNotificationPref(ctx, userID, string(body.Category), body.Enabled); err != nil {
		return oapi.UpdateNotificationPreference500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	t := true
	return oapi.UpdateNotificationPreference200JSONResponse{Ok: &t}, nil
}

// ── Auth (delegated to AuthProvider) ─────────────────────────────────
// These handlers delegate to h.Config.Auth which needs (w, r) — we retrieve
// the *http.Request from context (injected by withHTTPRequest middleware).
// We use per-endpoint raw response types whose Visit method calls through.

// startLoginRaw is a raw response that delegates writing to the auth provider.
type startLoginRaw struct{ fn func(w http.ResponseWriter) }

func (r startLoginRaw) VisitStartLoginResponse(w http.ResponseWriter) error {
	r.fn(w)
	return nil
}

// authCallbackRaw is a raw response that delegates writing to the auth provider.
type authCallbackRaw struct{ fn func(w http.ResponseWriter) }

func (r authCallbackRaw) VisitAuthCallbackResponse(w http.ResponseWriter) error {
	r.fn(w)
	return nil
}

// tokenExchangeRaw is a raw response that delegates writing to the auth provider.
type tokenExchangeRaw struct{ fn func(w http.ResponseWriter) }

func (r tokenExchangeRaw) VisitTokenExchangeResponse(w http.ResponseWriter) error {
	r.fn(w)
	return nil
}

// getSessionRaw is a raw response that delegates writing to the auth provider.
type getSessionRaw struct{ fn func(w http.ResponseWriter) }

func (r getSessionRaw) VisitGetSessionResponse(w http.ResponseWriter) error {
	r.fn(w)
	return nil
}

// logoutRaw is a raw response that delegates writing to the auth provider.
type logoutRaw struct{ fn func(w http.ResponseWriter) }

func (r logoutRaw) VisitLogoutResponse(w http.ResponseWriter) error {
	r.fn(w)
	return nil
}

// StartLogin delegates to the auth provider to begin the login flow.
func (h *Handlers) StartLogin(ctx context.Context, _ oapi.StartLoginRequestObject) (oapi.StartLoginResponseObject, error) {
	r := reqFromCtx(ctx)
	return startLoginRaw{fn: func(w http.ResponseWriter) {
		h.Config.Auth.StartLogin(w, r)
	}}, nil
}

// AuthCallback delegates to the auth provider to handle the OAuth callback.
func (h *Handlers) AuthCallback(ctx context.Context, _ oapi.AuthCallbackRequestObject) (oapi.AuthCallbackResponseObject, error) {
	r := reqFromCtx(ctx)
	return authCallbackRaw{fn: func(w http.ResponseWriter) {
		h.Config.Auth.HandleCallback(w, r)
	}}, nil
}

// TokenExchange delegates to the auth provider for JWT-to-session exchange.
func (h *Handlers) TokenExchange(ctx context.Context, _ oapi.TokenExchangeRequestObject) (oapi.TokenExchangeResponseObject, error) {
	r := reqFromCtx(ctx)
	return tokenExchangeRaw{fn: func(w http.ResponseWriter) {
		h.Config.Auth.TokenExchange(w, r)
	}}, nil
}

// GetSession delegates to the auth provider to return the current session.
func (h *Handlers) GetSession(ctx context.Context, _ oapi.GetSessionRequestObject) (oapi.GetSessionResponseObject, error) {
	r := reqFromCtx(ctx)
	return getSessionRaw{fn: func(w http.ResponseWriter) {
		h.Config.Auth.GetSession(w, r)
	}}, nil
}

// Logout delegates to the auth provider to end the session.
func (h *Handlers) Logout(ctx context.Context, _ oapi.LogoutRequestObject) (oapi.LogoutResponseObject, error) {
	r := reqFromCtx(ctx)
	return logoutRaw{fn: func(w http.ResponseWriter) {
		h.Config.Auth.Logout(w, r)
	}}, nil
}
