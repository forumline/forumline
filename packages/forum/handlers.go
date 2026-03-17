package forum

import (
	"context"
	"errors"
	"net/http"

	"github.com/forumline/forumline/backend/sse"
	"github.com/forumline/forumline/forum/oapi"
	"github.com/forumline/forumline/forum/service"
	"github.com/forumline/forumline/forum/store"
)

// Compile-time check that *Handlers satisfies the strict generated interface.
var _ oapi.StrictServerInterface = (*Handlers)(nil)

// Handlers holds dependencies for all forum API handlers.
// Config is injected at construction time and contains all pluggable
// dependencies (auth provider, file storage, database, etc.).
type Handlers struct {
	SSEHub          *sse.Hub
	Config          *Config
	Store           *store.Store
	ThreadSvc       *service.ThreadService
	PostSvc         *service.PostService
	ProfileSvc      *service.ProfileService
	ChatSvc         *service.ChatService
	AdminSvc        *service.AdminService
	NotificationSvc *service.NotificationService
	ProfileCache    *ProfileCache
}

// serviceErrMsg maps service-layer errors to (httpStatus, message) for
// building typed strict responses. Callers use the returned values to
// construct the appropriate oapi.*JSONResponse.
func serviceErrStatus(err error) (int, string) {
	var validationErr *service.ValidationError
	var notFoundErr *service.NotFoundError
	var forbiddenErr *service.ForbiddenError
	var conflictErr *service.ConflictError

	switch {
	case errors.As(err, &validationErr):
		return http.StatusBadRequest, validationErr.Msg
	case errors.As(err, &notFoundErr):
		return http.StatusNotFound, notFoundErr.Msg
	case errors.As(err, &forbiddenErr):
		return http.StatusForbidden, forbiddenErr.Msg
	case errors.As(err, &conflictErr):
		return http.StatusConflict, conflictErr.Msg
	default:
		return http.StatusInternalServerError, err.Error()
	}
}

// httpReqKey is the context key for the injected *http.Request.
type httpReqKey struct{}

// withHTTPRequest injects the *http.Request into the context so that auth
// delegate handlers (StartLogin, AuthCallback, etc.) can retrieve it.
func withHTTPRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), httpReqKey{}, r)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// reqFromCtx extracts the injected *http.Request from the context.
func reqFromCtx(ctx context.Context) *http.Request {
	r, _ := ctx.Value(httpReqKey{}).(*http.Request)
	return r
}
