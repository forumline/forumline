package forum

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/forumline/forumline/backend/httpkit"
	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/service"
	"github.com/forumline/forumline/forum/store"
)

// NewRouter creates the forum HTTP router with all endpoints wired up.
// The Config must have Auth, Storage, DB, and SSEHub set.
// ValkeyClient may be nil (rate limiting falls back to in-memory).
func NewRouter(cfg *Config) http.Handler {
	// Create layers
	s := store.New(cfg.DB)

	notifSvc := service.NewNotificationService(s, &service.NotificationConfig{
		ForumlineURL: cfg.ForumlineURL,
		ServiceKey:   cfg.ForumlineServiceKey,
	}, cfg.EventBus, cfg.Schema)
	threadSvc := service.NewThreadService(s)
	postSvc := service.NewPostService(s, notifSvc, cfg.EventBus, cfg.Schema)
	profileSvc := service.NewProfileService(s)
	chatSvc := service.NewChatService(s, cfg.EventBus, cfg.Schema)
	adminSvc := service.NewAdminService(s)

	h := &Handlers{
		SSEHub:          cfg.SSEHub,
		Config:          cfg,
		Store:           s,
		ThreadSvc:       threadSvc,
		PostSvc:         postSvc,
		ProfileSvc:      profileSvc,
		ChatSvc:         chatSvc,
		AdminSvc:        adminSvc,
		NotificationSvc: notifSvc,
		ProfileCache:    NewProfileCache(cfg.ValkeyClient, cfg.DB, 30*time.Second),
	}

	// Build the strict handler and wrapper — wrapper methods are http.HandlerFunc.
	strictHandler := oapi.NewStrictHandler(h, nil)
	w := oapi.ServerInterfaceWrapper{
		Handler: strictHandler,
		ErrorHandlerFunc: func(rw http.ResponseWriter, r *http.Request, err error) {
			http.Error(rw, err.Error(), http.StatusBadRequest)
		},
	}

	// Middleware
	auth := cfg.Auth.Middleware()
	chatRL := httpkit.UserRateLimitMiddleware(httpkit.NewValkeyRateLimiter(cfg.ValkeyClient, 60, time.Minute))
	writeRL := httpkit.UserRateLimitMiddleware(httpkit.NewValkeyRateLimiter(cfg.ValkeyClient, 20, time.Minute))
	uploadRL := httpkit.UserRateLimitMiddleware(httpkit.NewValkeyRateLimiter(cfg.ValkeyClient, 5, time.Minute))
	importRL := httpkit.UserRateLimitMiddleware(httpkit.NewValkeyRateLimiter(cfg.ValkeyClient, 3, time.Minute))
	authRL := httpkit.RateLimitMiddleware(httpkit.NewValkeyRateLimiter(cfg.ValkeyClient, 20, time.Minute))

	r := chi.NewRouter()

	// Inject *http.Request into context for auth delegate handlers
	// (StartLogin, AuthCallback, etc. need w/r from strict handler context).
	r.Use(withHTTPRequest)

	// ── Public routes (no auth) ──────────────────────────────────────────

	r.Get("/.well-known/forumline-manifest.json", w.GetManifest)
	r.Get("/api/config", w.GetConfig)

	// Categories & threads (public read)
	r.Get("/api/categories", w.ListCategories)
	r.Get("/api/categories/{slug}", w.GetCategoryBySlug)
	r.Get("/api/categories/{slug}/threads", w.ListThreadsByCategory)
	r.Get("/api/threads", w.ListThreads)
	r.Get("/api/threads/{id}", w.GetThread)
	r.Get("/api/threads/{id}/posts", w.ListPostsByThread)

	// Channels (public read)
	r.Get("/api/channels", w.ListChannels)
	r.Get("/api/channels/{slug}/messages", w.ListChatMessages)

	// Profiles (public read)
	r.Get("/api/profiles/batch", w.GetProfilesBatch)
	r.Get("/api/profiles/by-username/{username}", w.GetProfileByUsername)
	r.Get("/api/profiles/{id}", w.GetProfile)

	// Search (public)
	r.Get("/api/search/posts", w.SearchPosts)
	r.Get("/api/search/threads", w.SearchThreads)

	// User content (public read)
	r.Get("/api/users/{id}/posts", w.ListUserPosts)
	r.Get("/api/users/{id}/threads", w.ListUserThreads)

	// Voice (public read)
	r.Get("/api/voice-presence", w.ListVoicePresence)
	r.Get("/api/voice-rooms", w.ListVoiceRooms)

	// Session (cookie-based, internal auth check)
	r.Get("/api/forumline/auth/session", w.GetSession)
	r.Delete("/api/forumline/auth/session", w.Logout)

	// ── Auth endpoints (rate-limited, no session auth) ───────────────────

	r.Group(func(r chi.Router) {
		r.Use(authRL)
		r.Get("/api/forumline/auth", w.StartLogin)
		r.Get("/api/forumline/auth/callback", w.AuthCallback)
		r.Post("/api/forumline/auth/token-exchange", w.TokenExchange)
	})

	// ── Authenticated routes ─────────────────────────────────────────────

	r.Group(func(r chi.Router) {
		r.Use(auth)

		// Channel follows
		r.Get("/api/channel-follows", w.ListChannelFollows)
		r.Post("/api/channel-follows", w.FollowChannel)
		r.Delete("/api/channel-follows", w.UnfollowChannel)

		// Notification preferences
		r.Get("/api/notification-preferences", w.ListNotificationPreferences)
		r.Put("/api/notification-preferences", w.UpdateNotificationPreference)

		// Forumline notifications
		r.Get("/api/forumline/notifications", w.ListForumlineNotifications)
		r.Post("/api/forumline/notifications/read", w.MarkNotificationRead)
		r.Get("/api/forumline/notifications/stream", w.StreamNotifications)
		r.Get("/api/forumline/unread", w.GetUnreadCounts)

		// LiveKit
		r.Get("/api/livekit", w.GetLiveKitParticipants)
		r.Post("/api/livekit", w.GetLiveKitToken)

		// SSE streams
		r.Get("/api/threads/{id}/stream", w.StreamPosts)
		r.Get("/api/channels/{slug}/stream", w.StreamChatMessages)
		r.Get("/api/voice-presence/stream", w.StreamVoicePresence)

		// Bookmarks
		r.Get("/api/bookmarks", w.ListBookmarks)
		r.Get("/api/bookmarks/{threadId}/status", w.GetBookmarkStatus)
		r.Delete("/api/bookmarks/{threadId}", w.RemoveBookmark)
		r.Delete("/api/bookmarks/by-id/{id}", w.RemoveBookmarkById)

		// In-forum notifications
		r.Get("/api/notifications", w.ListNotifications)
		r.Post("/api/notifications/read-all", w.MarkAllNotificationsRead)

		// Voice presence
		r.Put("/api/voice-presence", w.SetVoicePresence)
		r.Delete("/api/voice-presence", w.ClearVoicePresence)

		// Profile writes
		r.Delete("/api/profiles/{id}/forumline-id", w.ClearForumlineId)

		// Admin
		r.Get("/api/admin/stats", w.GetAdminStats)
		r.Get("/api/admin/users", w.ListAdminUsers)
	})

	// ── Auth + write rate limit ──────────────────────────────────────────

	r.Group(func(r chi.Router) {
		r.Use(auth, writeRL)
		r.Post("/api/posts", w.CreatePost)
		r.Post("/api/threads", w.CreateThread)
		r.Patch("/api/threads/{id}", w.UpdateThread)
		r.Post("/api/bookmarks", w.AddBookmark)
		r.Put("/api/profiles/{id}", w.UpsertProfile)
	})

	// ── Auth + chat rate limit ───────────────────────────────────────────

	r.Group(func(r chi.Router) {
		r.Use(auth, chatRL)
		r.Post("/api/channels/{slug}/messages", w.SendChatMessage)
		r.Post("/api/channels/_by-id/{id}/messages", w.SendChatMessageByID)
	})

	// ── Auth + upload rate limit ─────────────────────────────────────────

	r.With(auth, uploadRL).Post("/api/avatars/upload", w.UploadAvatar)

	// ── Auth + import rate limit ─────────────────────────────────────────

	r.With(auth, importRL).Post("/api/admin/import", w.ImportData)

	return r
}
