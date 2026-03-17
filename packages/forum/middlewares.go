package forum

// middlewares.go: Per-operation middleware map. Maps oapi-codegen operation IDs
// to middleware stacks so the generated router can apply auth, rate limiting,
// etc. without hand-wiring every route.

import (
	"net/http"
	"time"

	"github.com/forumline/forumline/backend/httpkit"
	"github.com/forumline/forumline/forum/oapi"
)

// OperationMiddleware maps an oapi-codegen operation name to a list of
// middleware that should wrap that operation's handler.
type OperationMiddleware map[string][]oapi.MiddlewareFunc

// BuildOperationMiddleware constructs the per-operation middleware map from
// the forum Config. This replaces the per-route middleware wiring that was
// previously done inline in NewRouter.
func BuildOperationMiddleware(cfg *Config) OperationMiddleware {
	auth := cfg.Auth.Middleware()

	chatRL := httpkit.UserRateLimitMiddleware(httpkit.NewValkeyRateLimiter(cfg.ValkeyClient, 60, time.Minute))
	writeRL := httpkit.UserRateLimitMiddleware(httpkit.NewValkeyRateLimiter(cfg.ValkeyClient, 20, time.Minute))
	uploadRL := httpkit.UserRateLimitMiddleware(httpkit.NewValkeyRateLimiter(cfg.ValkeyClient, 5, time.Minute))
	importRL := httpkit.UserRateLimitMiddleware(httpkit.NewValkeyRateLimiter(cfg.ValkeyClient, 3, time.Minute))
	authRL := httpkit.RateLimitMiddleware(httpkit.NewValkeyRateLimiter(cfg.ValkeyClient, 20, time.Minute))

	m := OperationMiddleware{
		// ── Auth endpoints (rate-limited, no session auth) ───────────
		"StartLogin":    {authRL},
		"AuthCallback":  {authRL},
		"TokenExchange": {authRL},
		// GetSession and Logout are session-based; the generated wrapper
		// already marks them as cookieAuth-scoped but the existing code
		// doesn't apply auth middleware (session check is internal).

		// ── Channel follows (authenticated) ─────────────────────────
		"ListChannelFollows": {auth},
		"FollowChannel":      {auth},
		"UnfollowChannel":    {auth},

		// ── Notification preferences (authenticated) ────────────────
		"ListNotificationPreferences":  {auth},
		"UpdateNotificationPreference": {auth},

		// ── Forumline notifications (authenticated) ─────────────────
		"ListForumlineNotifications": {auth},
		"MarkNotificationRead":       {auth},
		"GetUnreadCounts":            {auth},
		"StreamNotifications":        {auth},

		// ── LiveKit (authenticated) ─────────────────────────────────
		"GetLiveKitToken":        {auth},
		"GetLiveKitParticipants": {auth},

		// ── SSE streams (authenticated) ─────────────────────────────
		"StreamPosts":         {auth},
		"StreamChatMessages":  {auth},
		"StreamVoicePresence": {auth},

		// ── Authenticated writes (auth + write rate limit) ──────────
		"CreateThread":             {auth, writeRL},
		"UpdateThread":             {auth, writeRL},
		"CreatePost":               {auth, writeRL},
		"AddBookmark":              {auth, writeRL},
		"UpsertProfile":            {auth, writeRL},
		"MarkAllNotificationsRead": {auth},

		// ── Chat sends (auth + chat rate limit) ─────────────────────
		"SendChatMessage":     {auth, chatRL},
		"SendChatMessageByID": {auth, chatRL},

		// ── Bookmarks (authenticated) ───────────────────────────────
		"ListBookmarks":     {auth},
		"GetBookmarkStatus": {auth},
		"RemoveBookmark":    {auth},
		"RemoveBookmarkById": {auth},

		// ── In-forum notifications (authenticated) ──────────────────
		"ListNotifications": {auth},

		// ── Voice presence (authenticated) ──────────────────────────
		"SetVoicePresence":   {auth},
		"ClearVoicePresence": {auth},

		// ── Profile writes (authenticated) ──────────────────────────
		"ClearForumlineId": {auth},

		// ── Upload (auth + upload rate limit) ───────────────────────
		"UploadAvatar": {auth, uploadRL},

		// ── Admin (authenticated) ───────────────────────────────────
		"GetAdminStats":  {auth},
		"ListAdminUsers": {auth},
		"ImportData":     {auth, importRL},
	}

	return m
}

// OperationRouter is an oapi.MiddlewareFunc that dispatches to per-operation
// middleware based on the operation name stored in the request context by
// the generated ServerInterfaceWrapper. This is used as the single global
// middleware in StdHTTPServerOptions.Middlewares.
//
// How it works: oapi-codegen v2's StdHTTP wrapper calls HandlerMiddlewares
// (i.e. the global Middlewares list) around each operation. We inspect which
// operation is being called by looking at the route pattern registered by
// the Go 1.22+ ServeMux (available via r.Pattern), and dispatch accordingly.
//
// However, the generated code does NOT set the operation name in context.
// Instead, we use a simpler approach: we build a middleware that wraps the
// handler, and for each route we know which operation it is because we
// registered the mapping ahead of time.
//
// Actually, the cleanest approach with oapi-codegen's StdHTTP server:
// we DON'T use the global Middlewares at all. Instead, we wrap each
// operation individually by implementing a custom ServerInterface wrapper.
// But that defeats the purpose.
//
// The pragmatic solution: since oapi-codegen's StdHTTP server applies the
// same Middlewares list to ALL operations, we need a different mechanism.
// We'll build a per-route wrapper using the mux pattern.

// NewPerOperationMiddleware returns an oapi.MiddlewareFunc that looks up
// per-operation middleware from the given map. It uses the request's mux
// pattern (Go 1.22+) to determine the operation.
func NewPerOperationMiddleware(opMiddleware OperationMiddleware) oapi.MiddlewareFunc {
	// Build a lookup from "METHOD /path" pattern to operation name.
	patternToOp := map[string]string{
		"GET /.well-known/forumline-manifest.json": "GetManifest",
		"POST /api/admin/import":                   "ImportData",
		"GET /api/admin/stats":                     "GetAdminStats",
		"GET /api/admin/users":                     "ListAdminUsers",
		"POST /api/avatars/upload":                 "UploadAvatar",
		"GET /api/bookmarks":                       "ListBookmarks",
		"POST /api/bookmarks":                      "AddBookmark",
		"DELETE /api/bookmarks/by-id/{id}":         "RemoveBookmarkById",
		"DELETE /api/bookmarks/{threadId}":         "RemoveBookmark",
		"GET /api/bookmarks/{threadId}/status":     "GetBookmarkStatus",
		"GET /api/categories":                      "ListCategories",
		"GET /api/categories/{slug}":               "GetCategoryBySlug",
		"GET /api/categories/{slug}/threads":       "ListThreadsByCategory",
		"DELETE /api/channel-follows":              "UnfollowChannel",
		"GET /api/channel-follows":                 "ListChannelFollows",
		"POST /api/channel-follows":                "FollowChannel",
		"GET /api/channels":                        "ListChannels",
		"POST /api/channels/_by-id/{id}/messages":  "SendChatMessageByID",
		"GET /api/channels/{slug}/messages":        "ListChatMessages",
		"POST /api/channels/{slug}/messages":       "SendChatMessage",
		"GET /api/channels/{slug}/stream":          "StreamChatMessages",
		"GET /api/config":                          "GetConfig",
		"GET /api/forumline/auth":                  "StartLogin",
		"GET /api/forumline/auth/callback":         "AuthCallback",
		"DELETE /api/forumline/auth/session":       "Logout",
		"GET /api/forumline/auth/session":          "GetSession",
		"POST /api/forumline/auth/token-exchange":  "TokenExchange",
		"GET /api/forumline/notifications":         "ListForumlineNotifications",
		"POST /api/forumline/notifications/read":   "MarkNotificationRead",
		"GET /api/forumline/notifications/stream":  "StreamNotifications",
		"GET /api/forumline/unread":                "GetUnreadCounts",
		"GET /api/livekit":                         "GetLiveKitParticipants",
		"POST /api/livekit":                        "GetLiveKitToken",
		"GET /api/notification-preferences":        "ListNotificationPreferences",
		"PUT /api/notification-preferences":        "UpdateNotificationPreference",
		"GET /api/notifications":                   "ListNotifications",
		"POST /api/notifications/read-all":         "MarkAllNotificationsRead",
		"POST /api/posts":                          "CreatePost",
		"GET /api/profiles/batch":                  "GetProfilesBatch",
		"GET /api/profiles/by-username/{username}": "GetProfileByUsername",
		"GET /api/profiles/{id}":                   "GetProfile",
		"PUT /api/profiles/{id}":                   "UpsertProfile",
		"DELETE /api/profiles/{id}/forumline-id":   "ClearForumlineId",
		"GET /api/search/posts":                    "SearchPosts",
		"GET /api/search/threads":                  "SearchThreads",
		"GET /api/threads":                         "ListThreads",
		"POST /api/threads":                        "CreateThread",
		"GET /api/threads/{id}":                    "GetThread",
		"PATCH /api/threads/{id}":                  "UpdateThread",
		"GET /api/threads/{id}/posts":              "ListPostsByThread",
		"GET /api/threads/{id}/stream":             "StreamPosts",
		"GET /api/users/{id}/posts":                "ListUserPosts",
		"GET /api/users/{id}/threads":              "ListUserThreads",
		"DELETE /api/voice-presence":               "ClearVoicePresence",
		"GET /api/voice-presence":                  "ListVoicePresence",
		"PUT /api/voice-presence":                  "SetVoicePresence",
		"GET /api/voice-presence/stream":           "StreamVoicePresence",
		"GET /api/voice-rooms":                     "ListVoiceRooms",
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pattern := r.Pattern
			if opName, ok := patternToOp[pattern]; ok {
				if mws, hasMW := opMiddleware[opName]; hasMW {
					// Wrap the next handler in the operation-specific middleware
					// chain (applied right-to-left so the first in the list runs first).
					h := next
					for i := len(mws) - 1; i >= 0; i-- {
						h = mws[i](h)
					}
					h.ServeHTTP(w, r)
					return
				}
			}
			// No per-operation middleware — pass through.
			next.ServeHTTP(w, r)
		})
	}
}
