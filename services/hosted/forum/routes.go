package forum

import (
	"net/http"
	"time"

	"github.com/forumline/forumline/services/hosted/forum/service"
	"github.com/forumline/forumline/services/hosted/forum/store"
	"github.com/redis/go-redis/v9"
	shared "github.com/forumline/forumline/shared-go"
)

func NewRouter(pool shared.DB, sseHub *shared.SSEHub, cfg *Config, valkey *redis.Client) *http.ServeMux {
	mux := http.NewServeMux()

	auth := shared.AuthMiddleware

	// Create layers
	s := store.New(pool)

	notifSvc := service.NewNotificationService(s, &service.NotificationConfig{
		ForumlineURL:          cfg.ForumlineURL,
		ForumlineClientID:     cfg.ZitadelClientID,
		ForumlineClientSecret: cfg.ZitadelClientSecret,
	})
	threadSvc := service.NewThreadService(s)
	postSvc := service.NewPostService(s, notifSvc)
	profileSvc := service.NewProfileService(s)
	chatSvc := service.NewChatService(s)
	adminSvc := service.NewAdminService(s)

	h := &Handlers{
		SSEHub:          sseHub,
		Config:          cfg,
		Store:           s,
		ThreadSvc:       threadSvc,
		PostSvc:         postSvc,
		ProfileSvc:      profileSvc,
		ChatSvc:         chatSvc,
		AdminSvc:        adminSvc,
		NotificationSvc: notifSvc,
		ProfileCache:    NewProfileCache(valkey, pool, 30*time.Second),
	}

	// Rate limiters (per-user for authenticated, per-IP for public/auth)
	chatRL := shared.UserRateLimitMiddleware(shared.NewValkeyRateLimiter(valkey, 60, time.Minute))   // 60 msgs/min
	writeRL := shared.UserRateLimitMiddleware(shared.NewValkeyRateLimiter(valkey, 20, time.Minute))  // 20 creates/min
	uploadRL := shared.UserRateLimitMiddleware(shared.NewValkeyRateLimiter(valkey, 5, time.Minute))  // 5 uploads/min
	importRL := shared.UserRateLimitMiddleware(shared.NewValkeyRateLimiter(valkey, 3, time.Minute))  // 3 imports/min
	authRL := shared.RateLimitMiddleware(shared.NewValkeyRateLimiter(valkey, 20, time.Minute))       // 20 auth attempts/min per IP

	// Channel follows (authenticated)
	mux.Handle("GET /api/channel-follows", shared.Use(h.HandleChannelFollows, auth))
	mux.Handle("POST /api/channel-follows", shared.Use(h.HandleChannelFollows, auth))
	mux.Handle("DELETE /api/channel-follows", shared.Use(h.HandleChannelFollows, auth))

	// Notification preferences (authenticated)
	mux.Handle("GET /api/notification-preferences", shared.Use(h.HandleNotificationPreferences, auth))
	mux.Handle("PUT /api/notification-preferences", shared.Use(h.HandleNotificationPreferences, auth))

	// Forumline OAuth (IP-based rate limit on auth endpoints)
	mux.Handle("GET /api/forumline/auth", shared.Use(h.HandleForumlineAuth, authRL))
	mux.Handle("POST /api/forumline/auth", shared.Use(h.HandleForumlineAuth, authRL))
	mux.Handle("GET /api/forumline/auth/callback", shared.Use(h.HandleForumlineCallback, authRL))
	mux.Handle("GET /api/forumline/auth/forumline-token", shared.Use(h.HandleForumlineToken, authRL))
	mux.HandleFunc("GET /api/forumline/auth/session", h.HandleForumlineSession)
	mux.HandleFunc("DELETE /api/forumline/auth/session", h.HandleForumlineSession)

	// Forumline notifications (authenticated)
	mux.Handle("GET /api/forumline/notifications", shared.Use(h.HandleNotifications, auth))
	mux.Handle("POST /api/forumline/notifications/read", shared.Use(h.HandleNotificationRead, auth))
	mux.Handle("GET /api/forumline/unread", shared.Use(h.HandleUnread, auth))
	mux.Handle("GET /api/forumline/notifications/stream", shared.Use(h.HandleNotificationStream, auth))

	// LiveKit (authenticated)
	mux.Handle("POST /api/livekit", shared.Use(h.HandleLiveKitToken, auth))
	mux.Handle("GET /api/livekit", shared.Use(h.HandleLiveKitParticipants, auth))

	// ================================================================
	// Data endpoints (Phase B)
	// ================================================================

	// Forum config (public)
	mux.HandleFunc("GET /api/config", h.HandleConfig)

	// Static/config (public)
	mux.HandleFunc("GET /api/categories", h.HandleCategories)
	mux.HandleFunc("GET /api/categories/{slug}", h.HandleCategoryBySlug)
	mux.HandleFunc("GET /api/channels", h.HandleChannels)
	mux.HandleFunc("GET /api/voice-rooms", h.HandleVoiceRooms)

	// Threads (public reads)
	mux.HandleFunc("GET /api/threads", h.HandleThreads)
	mux.HandleFunc("GET /api/threads/{id}", h.HandleThread)
	mux.HandleFunc("GET /api/categories/{slug}/threads", h.HandleThreadsByCategory)
	mux.HandleFunc("GET /api/users/{id}/threads", h.HandleUserThreads)
	mux.HandleFunc("GET /api/search/threads", h.HandleSearchThreads)

	// Posts (public reads + stream)
	mux.HandleFunc("GET /api/threads/{id}/posts", h.HandlePosts)
	mux.Handle("GET /api/threads/{id}/stream", shared.Use(h.HandlePostStream, auth))
	mux.HandleFunc("GET /api/users/{id}/posts", h.HandleUserPosts)
	mux.HandleFunc("GET /api/search/posts", h.HandleSearchPosts)

	// Profiles (public reads)
	mux.HandleFunc("GET /api/profiles/batch", h.HandleProfilesBatch)
	mux.HandleFunc("GET /api/profiles/by-username/{username}", h.HandleProfileByUsername)
	mux.HandleFunc("GET /api/profiles/{id}", h.HandleProfile)

	// Chat messages (public read)
	mux.HandleFunc("GET /api/channels/{slug}/messages", h.HandleChatMessages)

	// Voice presence (public read)
	mux.HandleFunc("GET /api/voice-presence", h.HandleVoicePresence)

	// Authenticated data endpoints (rate-limited writes)
	mux.Handle("POST /api/threads", shared.Use(h.HandleCreateThread, auth, writeRL))
	mux.Handle("PATCH /api/threads/{id}", shared.Use(h.HandleUpdateThread, auth, writeRL))
	mux.Handle("POST /api/posts", shared.Use(h.HandleCreatePost, auth, writeRL))
	mux.Handle("POST /api/channels/{slug}/messages", shared.Use(h.HandleSendChatMessage, auth, chatRL))
	mux.Handle("POST /api/channels/_by-id/{id}/messages", shared.Use(h.HandleSendChatMessageByID, auth, chatRL))
	mux.Handle("GET /api/channels/{slug}/stream", shared.Use(h.HandleChatStream, auth))
	mux.Handle("GET /api/bookmarks", shared.Use(h.HandleBookmarks, auth))
	mux.Handle("GET /api/bookmarks/{threadId}/status", shared.Use(h.HandleBookmarkStatus, auth))
	mux.Handle("POST /api/bookmarks", shared.Use(h.HandleAddBookmark, auth, writeRL))
	mux.Handle("DELETE /api/bookmarks/{threadId}", shared.Use(h.HandleRemoveBookmark, auth))
	mux.Handle("DELETE /api/bookmarks/by-id/{id}", shared.Use(h.HandleRemoveBookmarkByID, auth))
	mux.Handle("GET /api/notifications", shared.Use(h.HandleNotificationsData, auth))
	mux.Handle("POST /api/notifications/read-all", shared.Use(h.HandleMarkAllNotificationsRead, auth))
	mux.Handle("PUT /api/profiles/{id}", shared.Use(h.HandleUpsertProfile, auth, writeRL))
	mux.Handle("DELETE /api/profiles/{id}/forumline-id", shared.Use(h.HandleClearForumlineID, auth))
	mux.Handle("PUT /api/voice-presence", shared.Use(h.HandleSetVoicePresence, auth))
	mux.Handle("DELETE /api/voice-presence", shared.Use(h.HandleClearVoicePresence, auth))
	mux.Handle("GET /api/voice-presence/stream", shared.Use(h.HandleVoicePresenceStream, auth))
	mux.Handle("POST /api/avatars/upload", shared.Use(h.HandleAvatarUpload, auth, uploadRL))
	mux.Handle("GET /api/admin/stats", shared.Use(h.HandleAdminStats, auth))
	mux.Handle("GET /api/admin/users", shared.Use(h.HandleAdminUsers, auth))
	mux.Handle("POST /api/admin/import", shared.Use(h.HandleImport, auth, importRL))

	// Forumline manifest (discovery)
	mux.HandleFunc("GET /.well-known/forumline-manifest.json", h.HandleManifest)

	return mux
}
