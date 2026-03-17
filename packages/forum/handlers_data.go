package forum

import (
	"context"
	"strings"

	"github.com/forumline/forumline/backend/auth"
	"github.com/forumline/forumline/forum/oapi"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ============================================================================
// Static / Config endpoints (public)
// ============================================================================

// ListCategories handles GET /api/categories
func (h *Handlers) ListCategories(ctx context.Context, _ oapi.ListCategoriesRequestObject) (oapi.ListCategoriesResponseObject, error) {
	categories, err := h.Store.ListCategories(ctx)
	if err != nil {
		return oapi.ListCategories500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListCategories200JSONResponse(categories), nil
}

// GetCategoryBySlug handles GET /api/categories/{slug}
func (h *Handlers) GetCategoryBySlug(ctx context.Context, request oapi.GetCategoryBySlugRequestObject) (oapi.GetCategoryBySlugResponseObject, error) {
	cat, err := h.Store.GetCategoryBySlug(ctx, request.Slug)
	if err != nil {
		return oapi.GetCategoryBySlug404JSONResponse{NotFoundJSONResponse: oapi.NotFoundJSONResponse{Error: "category not found"}}, nil
	}
	return oapi.GetCategoryBySlug200JSONResponse(*cat), nil
}

// ListChannels handles GET /api/channels
func (h *Handlers) ListChannels(ctx context.Context, _ oapi.ListChannelsRequestObject) (oapi.ListChannelsResponseObject, error) {
	channels, err := h.Store.ListChannels(ctx)
	if err != nil {
		return oapi.ListChannels500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListChannels200JSONResponse(channels), nil
}

// ListVoiceRooms handles GET /api/voice-rooms
func (h *Handlers) ListVoiceRooms(ctx context.Context, _ oapi.ListVoiceRoomsRequestObject) (oapi.ListVoiceRoomsResponseObject, error) {
	rooms, err := h.Store.ListVoiceRooms(ctx)
	if err != nil {
		return oapi.ListVoiceRooms500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListVoiceRooms200JSONResponse(rooms), nil
}

// ============================================================================
// Threads
// ============================================================================

// ListThreads handles GET /api/threads
func (h *Handlers) ListThreads(ctx context.Context, request oapi.ListThreadsRequestObject) (oapi.ListThreadsResponseObject, error) {
	limit := 20
	if request.Params.Limit != nil {
		l := *request.Params.Limit
		if l > 0 && l <= 100 {
			limit = l
		}
	}

	threads, err := h.ThreadSvc.List(ctx, limit)
	if err != nil {
		status, msg := serviceErrStatus(err)
		if status == 500 {
			return oapi.ListThreads500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: msg}}, nil
		}
		return oapi.ListThreads500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: msg}}, nil
	}
	return oapi.ListThreads200JSONResponse(threads), nil
}

// GetThread handles GET /api/threads/{id}
func (h *Handlers) GetThread(ctx context.Context, request oapi.GetThreadRequestObject) (oapi.GetThreadResponseObject, error) {
	t, err := h.ThreadSvc.Get(ctx, request.Id.String())
	if err != nil {
		_, msg := serviceErrStatus(err)
		return oapi.GetThread404JSONResponse{NotFoundJSONResponse: oapi.NotFoundJSONResponse{Error: msg}}, nil
	}
	return oapi.GetThread200JSONResponse(*t), nil
}

// ListThreadsByCategory handles GET /api/categories/{slug}/threads
func (h *Handlers) ListThreadsByCategory(ctx context.Context, request oapi.ListThreadsByCategoryRequestObject) (oapi.ListThreadsByCategoryResponseObject, error) {
	threads, err := h.ThreadSvc.ListByCategory(ctx, request.Slug)
	if err != nil {
		return oapi.ListThreadsByCategory500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListThreadsByCategory200JSONResponse(threads), nil
}

// ListUserThreads handles GET /api/users/{id}/threads
func (h *Handlers) ListUserThreads(ctx context.Context, request oapi.ListUserThreadsRequestObject) (oapi.ListUserThreadsResponseObject, error) {
	threads, err := h.ThreadSvc.ListByUser(ctx, request.Id.String())
	if err != nil {
		return oapi.ListUserThreads500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListUserThreads200JSONResponse(threads), nil
}

// SearchThreads handles GET /api/search/threads?q=
func (h *Handlers) SearchThreads(ctx context.Context, request oapi.SearchThreadsRequestObject) (oapi.SearchThreadsResponseObject, error) {
	q := strings.TrimSpace(request.Params.Q)
	threads, err := h.ThreadSvc.Search(ctx, q)
	if err != nil {
		return oapi.SearchThreads500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.SearchThreads200JSONResponse(threads), nil
}

// ============================================================================
// Posts
// ============================================================================

// ListPostsByThread handles GET /api/threads/{id}/posts
func (h *Handlers) ListPostsByThread(ctx context.Context, request oapi.ListPostsByThreadRequestObject) (oapi.ListPostsByThreadResponseObject, error) {
	posts, err := h.PostSvc.ListByThread(ctx, request.Id.String())
	if err != nil {
		return oapi.ListPostsByThread500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListPostsByThread200JSONResponse(posts), nil
}

// ListUserPosts handles GET /api/users/{id}/posts
func (h *Handlers) ListUserPosts(ctx context.Context, request oapi.ListUserPostsRequestObject) (oapi.ListUserPostsResponseObject, error) {
	posts, err := h.PostSvc.ListByUser(ctx, request.Id.String())
	if err != nil {
		return oapi.ListUserPosts500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListUserPosts200JSONResponse(posts), nil
}

// SearchPosts handles GET /api/search/posts?q=
func (h *Handlers) SearchPosts(ctx context.Context, request oapi.SearchPostsRequestObject) (oapi.SearchPostsResponseObject, error) {
	q := strings.TrimSpace(request.Params.Q)
	posts, err := h.PostSvc.Search(ctx, q)
	if err != nil {
		return oapi.SearchPosts500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.SearchPosts200JSONResponse(posts), nil
}

// ============================================================================
// Profiles
// ============================================================================

// GetProfile handles GET /api/profiles/{id}
func (h *Handlers) GetProfile(ctx context.Context, request oapi.GetProfileRequestObject) (oapi.GetProfileResponseObject, error) {
	p, err := h.ProfileSvc.Get(ctx, request.Id.String())
	if err != nil {
		return oapi.GetProfile404JSONResponse{NotFoundJSONResponse: oapi.NotFoundJSONResponse{Error: "profile not found"}}, nil
	}
	return oapi.GetProfile200JSONResponse(*p), nil
}

// GetProfileByUsername handles GET /api/profiles/by-username/{username}
func (h *Handlers) GetProfileByUsername(ctx context.Context, request oapi.GetProfileByUsernameRequestObject) (oapi.GetProfileByUsernameResponseObject, error) {
	p, err := h.ProfileSvc.GetByUsername(ctx, request.Username)
	if err != nil {
		return oapi.GetProfileByUsername404JSONResponse{NotFoundJSONResponse: oapi.NotFoundJSONResponse{Error: "profile not found"}}, nil
	}
	return oapi.GetProfileByUsername200JSONResponse(*p), nil
}

// GetProfilesBatch handles GET /api/profiles/batch?ids=id1,id2,...
func (h *Handlers) GetProfilesBatch(ctx context.Context, request oapi.GetProfilesBatchRequestObject) (oapi.GetProfilesBatchResponseObject, error) {
	idsParam := request.Params.Ids
	if idsParam == "" {
		return oapi.GetProfilesBatch200JSONResponse([]oapi.Profile{}), nil
	}
	ids := strings.Split(idsParam, ",")
	profiles, err := h.ProfileSvc.GetBatch(ctx, ids)
	if err != nil {
		return oapi.GetProfilesBatch500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.GetProfilesBatch200JSONResponse(profiles), nil
}

// ============================================================================
// Chat Messages
// ============================================================================

// ListChatMessages handles GET /api/channels/{slug}/messages
func (h *Handlers) ListChatMessages(ctx context.Context, request oapi.ListChatMessagesRequestObject) (oapi.ListChatMessagesResponseObject, error) {
	messages, err := h.ChatSvc.ListMessages(ctx, request.Slug)
	if err != nil {
		return oapi.ListChatMessages500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListChatMessages200JSONResponse(messages), nil
}

// ============================================================================
// Voice Presence
// ============================================================================

// ListVoicePresence handles GET /api/voice-presence
func (h *Handlers) ListVoicePresence(ctx context.Context, _ oapi.ListVoicePresenceRequestObject) (oapi.ListVoicePresenceResponseObject, error) {
	presence, err := h.Store.ListVoicePresence(ctx)
	if err != nil {
		return oapi.ListVoicePresence500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListVoicePresence200JSONResponse(presence), nil
}

// ============================================================================
// Bookmarks
// ============================================================================

// ListBookmarks handles GET /api/bookmarks
func (h *Handlers) ListBookmarks(ctx context.Context, _ oapi.ListBookmarksRequestObject) (oapi.ListBookmarksResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)
	bookmarks, err := h.Store.ListBookmarks(ctx, userID)
	if err != nil {
		return oapi.ListBookmarks500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListBookmarks200JSONResponse(bookmarks), nil
}

// GetBookmarkStatus handles GET /api/bookmarks/{threadId}/status
func (h *Handlers) GetBookmarkStatus(ctx context.Context, request oapi.GetBookmarkStatusRequestObject) (oapi.GetBookmarkStatusResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)

	id, err := h.Store.GetBookmarkStatus(ctx, userID, request.ThreadId.String())
	if err != nil || id == nil {
		f := false
		return oapi.GetBookmarkStatus200JSONResponse{Bookmarked: &f}, nil
	}
	t := true
	// Parse the bookmark ID string into a UUID
	var bookmarkUUID openapi_types.UUID
	if parseErr := bookmarkUUID.UnmarshalText([]byte(*id)); parseErr != nil {
		f := false
		return oapi.GetBookmarkStatus200JSONResponse{Bookmarked: &f}, nil
	}
	return oapi.GetBookmarkStatus200JSONResponse{Bookmarked: &t, Id: &bookmarkUUID}, nil
}

// ============================================================================
// Notifications (data read)
// ============================================================================

// ListNotifications handles GET /api/notifications (data provider version)
func (h *Handlers) ListNotifications(ctx context.Context, _ oapi.ListNotificationsRequestObject) (oapi.ListNotificationsResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)
	notifications, err := h.Store.ListNotifications(ctx, userID, 20)
	if err != nil {
		return oapi.ListNotifications500JSONResponse{InternalErrorJSONResponse: oapi.InternalErrorJSONResponse{Error: err.Error()}}, nil
	}
	return oapi.ListNotifications200JSONResponse(notifications), nil
}

// ============================================================================
// Admin
// ============================================================================

// GetAdminStats handles GET /api/admin/stats
func (h *Handlers) GetAdminStats(ctx context.Context, _ oapi.GetAdminStatsRequestObject) (oapi.GetAdminStatsResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)
	stats, err := h.AdminSvc.GetStats(ctx, userID)
	if err != nil {
		status, msg := serviceErrStatus(err)
		if status == 403 {
			return oapi.GetAdminStats403JSONResponse{ForbiddenJSONResponse: oapi.ForbiddenJSONResponse{Error: msg}}, nil
		}
		return oapi.GetAdminStats403JSONResponse{ForbiddenJSONResponse: oapi.ForbiddenJSONResponse{Error: msg}}, nil
	}
	return oapi.GetAdminStats200JSONResponse(*stats), nil
}

// ListAdminUsers handles GET /api/admin/users
func (h *Handlers) ListAdminUsers(ctx context.Context, _ oapi.ListAdminUsersRequestObject) (oapi.ListAdminUsersResponseObject, error) {
	userID := auth.UserIDFromContext(ctx)
	profiles, err := h.AdminSvc.ListUsers(ctx, userID)
	if err != nil {
		status, msg := serviceErrStatus(err)
		if status == 403 {
			return oapi.ListAdminUsers403JSONResponse{ForbiddenJSONResponse: oapi.ForbiddenJSONResponse{Error: msg}}, nil
		}
		return oapi.ListAdminUsers403JSONResponse{ForbiddenJSONResponse: oapi.ForbiddenJSONResponse{Error: msg}}, nil
	}
	return oapi.ListAdminUsers200JSONResponse(profiles), nil
}
