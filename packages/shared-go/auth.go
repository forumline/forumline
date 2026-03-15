package shared

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/zitadel/zitadel-go/v3/pkg/authorization"
	"github.com/zitadel/zitadel-go/v3/pkg/authorization/oauth"
	"github.com/zitadel/zitadel-go/v3/pkg/http/middleware"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
)

type contextKey string

const UserIDKey contextKey = "userID"

// zitadelMW is the Zitadel HTTP middleware interceptor, initialized on first use.
var zitadelMW *middleware.Interceptor[*oauth.IntrospectionContext]

// InitAuth initializes the Zitadel authorization middleware.
// Must be called once at startup before any requests are served.
// Reads ZITADEL_URL and ZITADEL_CLIENT_ID from environment.
func InitAuth(ctx context.Context) error {
	zitadelURL := os.Getenv("ZITADEL_URL")
	if zitadelURL == "" {
		return fmt.Errorf("ZITADEL_URL is not set")
	}
	clientID := os.Getenv("ZITADEL_CLIENT_ID")
	if clientID == "" {
		return fmt.Errorf("ZITADEL_CLIENT_ID is not set")
	}

	authZ, err := authorization.New(ctx,
		zitadel.New(zitadelURL),
		oauth.DefaultJWTAuthorization(clientID),
	)
	if err != nil {
		return fmt.Errorf("init zitadel authorization: %w", err)
	}

	zitadelMW = middleware.New(authZ)
	return nil
}

// AuthMiddleware validates the JWT from the Authorization header or access_token
// query parameter, then sets the user ID in the request context.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Promote access_token query param to Authorization header for SSE/EventSource
		r = promoteQueryToken(r)

		// Use Zitadel's RequireAuthorization middleware
		zitadelMW.RequireAuthorization()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := authorization.UserID(r.Context())
			if userID == "" {
				http.Error(w, `{"error":"invalid token: no subject"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})).ServeHTTP(w, r)
	})
}

// OptionalAuthMiddleware extracts the JWT if present but doesn't require it.
func OptionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = promoteQueryToken(r)

		// Use Zitadel's CheckAuthorization (doesn't reject unauthenticated requests)
		zitadelMW.CheckAuthorization()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if userID := authorization.UserID(r.Context()); userID != "" {
				ctx := context.WithValue(r.Context(), UserIDKey, userID)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})).ServeHTTP(w, r)
	})
}

// UserIDFromContext returns the authenticated user ID from the request context.
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

// promoteQueryToken copies the access_token query parameter to the Authorization
// header if no Authorization header is present. This supports SSE/EventSource
// connections which cannot set custom headers.
func promoteQueryToken(r *http.Request) *http.Request {
	if r.Header.Get("Authorization") != "" {
		return r
	}
	if token := r.URL.Query().Get("access_token"); token != "" {
		r2 := r.Clone(r.Context())
		r2.Header.Set("Authorization", "Bearer "+token)
		return r2
	}
	return r
}

// extractToken extracts the bearer token from the request for non-middleware use.
func extractToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	if token := r.URL.Query().Get("access_token"); token != "" {
		return token
	}
	return ""
}

// MustInitAuth calls InitAuth and exits on failure. Convenience for main().
func MustInitAuth(ctx context.Context) {
	if err := InitAuth(ctx); err != nil {
		slog.Error("failed to initialize auth", "error", err)
		os.Exit(1)
	}
}
