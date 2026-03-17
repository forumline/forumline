// Package forum provides a reusable forum engine that can run in hosted
// (multi-tenant) or standalone (single-tenant) mode. The forum logic is
// identical in both modes — only the injected AuthProvider, FileStorage,
// and database connection differ.
package forum

import (
	"context"
	"net/http"
)

// UserIdentity holds the authenticated user's identity from the auth provider.
// This is what the forum uses to create or link a local profile.
type UserIdentity struct {
	// ProviderID is the unique user ID from the identity provider.
	// For Forumline-hosted forums, this is the Zitadel subject ID.
	// For standalone forums, this is whatever the OIDC provider returns as "sub".
	ProviderID string

	// Username, DisplayName, AvatarURL are profile hints from the provider.
	// The forum uses these to bootstrap local profiles on first login.
	Username    string
	DisplayName string
	AvatarURL   string
}

// AuthProvider abstracts authentication so the forum engine works with any
// identity system. Hosted forums use the Forumline identity service
// (id.forumline.net); standalone forums can use any OIDC provider directly.
//
// The provider is responsible for:
//   - Validating tokens and setting the user ID in the request context
//     via auth.UserIDKey (from packages/backend/auth)
//   - Handling the full login flow (redirect → callback → session)
//   - Managing session cookies
type AuthProvider interface {
	// Middleware returns HTTP middleware that validates the access token
	// (from Authorization header or access_token query param) and sets
	// the user ID in the request context using auth.UserIDKey.
	//
	// Requests without a valid token MUST receive a 401 response.
	// This middleware is applied to all authenticated endpoints.
	Middleware() func(http.Handler) http.Handler

	// StartLogin handles GET /api/auth/login.
	// Redirects the user to the identity provider's authorization endpoint.
	StartLogin(w http.ResponseWriter, r *http.Request)

	// HandleCallback handles GET /api/auth/callback.
	// Processes the OAuth/OIDC callback from the identity provider.
	// Must validate the state parameter, exchange the auth code for tokens,
	// create/link the local forum profile, set a session cookie, and redirect
	// the user back to the forum.
	HandleCallback(w http.ResponseWriter, r *http.Request)

	// TokenExchange handles POST /api/auth/token-exchange.
	// Validates a JWT passed via postMessage from an embedding app (the
	// "invisible handshake" for iframe-based browsing). Returns user info
	// and a local session without a redirect flow.
	//
	// Request body: {"token": "..."}
	// Response: {"access_token": "...", "local_user_id": "...", "user": {...}}
	//
	// If the forum is not embedded in an app, this can return 501.
	TokenExchange(w http.ResponseWriter, r *http.Request)

	// GetSession handles GET /api/auth/session.
	// Returns the current session info from cookies, or null if not logged in.
	GetSession(w http.ResponseWriter, r *http.Request)

	// Logout handles DELETE /api/auth/session.
	// Clears the session cookie and returns success.
	Logout(w http.ResponseWriter, r *http.Request)

	// CreateOrLinkUser creates a local forum profile for the given identity,
	// or links to an existing profile if the provider ID is already known.
	// Returns the local user ID.
	//
	// This is called by the auth flow handlers (callback, token exchange)
	// and needs access to the forum's profile store.
	CreateOrLinkUser(ctx context.Context, identity *UserIdentity) (localUserID string, err error)
}
