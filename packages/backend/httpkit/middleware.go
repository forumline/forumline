package httpkit

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/forumline/forumline/backend/auth"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
)

// CORSMiddleware handles CORS for the API.
// Supports exact origins and wildcard subdomain patterns like "https://*.forumline.net".
func CORSMiddleware(next http.Handler) http.Handler {
	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS") // comma-separated
	if allowedOrigins == "" {
		allowedOrigins = "https://app.forumline.net"
	}

	// Parse the allowed origins list once at init time.
	var exact []string
	type wildcardPattern struct {
		prefix string // "https://"
		suffix string // e.g. ".forumline.net"
	}
	var wildcards []wildcardPattern

	allowAll := false
	for _, o := range strings.Split(allowedOrigins, ",") {
		o = strings.TrimSpace(o)
		if o == "*" {
			allowAll = true
		} else if strings.HasPrefix(o, "https://*.") {
			suffix := o[len("https://*"):] // e.g. ".forumline.net"
			wildcards = append(wildcards, wildcardPattern{prefix: "https://", suffix: suffix})
		} else {
			exact = append(exact, o)
		}
	}

	return cors.Handler(cors.Options{
		AllowOriginFunc: func(_ *http.Request, origin string) bool {
			if allowAll {
				return true
			}
			for _, o := range exact {
				if o == origin {
					return true
				}
			}
			for _, w := range wildcards {
				if strings.HasPrefix(origin, w.prefix) && strings.HasSuffix(origin, w.suffix) {
					host := origin[len(w.prefix):]
					host = strings.TrimSuffix(host, w.suffix)
					if host != "" && !strings.Contains(host, ".") {
						return true
					}
				}
			}
			return false
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Forumline-ID"},
		AllowCredentials: true,
		MaxAge:           86400,
	})(next)
}

// SecurityHeaders adds standard security headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// ipKeyFunc returns the appropriate httprate key function based on TRUST_PROXY.
func ipKeyFunc() httprate.KeyFunc {
	if os.Getenv("TRUST_PROXY") == "true" {
		return httprate.KeyByRealIP
	}
	return httprate.KeyByIP
}

// KeyByUserID returns the authenticated user's ID as the rate limit key.
// Falls back to IP-based keying if no user is in the context.
func KeyByUserID(r *http.Request) (string, error) {
	if uid := auth.UserIDFromContext(r.Context()); uid != "" {
		return uid, nil
	}
	return ipKeyFunc()(r)
}

// IPRateLimit returns chi-compatible middleware that rate limits per client IP.
// Respects TRUST_PROXY=true for X-Forwarded-For / X-Real-IP extraction.
func IPRateLimit(requestLimit int, windowLength time.Duration) func(http.Handler) http.Handler {
	return httprate.Limit(requestLimit, windowLength,
		httprate.WithKeyFuncs(ipKeyFunc()),
		httprate.WithLimitHandler(rateLimitHandler),
	)
}

// UserRateLimit returns chi-compatible middleware that rate limits per
// authenticated user ID. Falls back to IP-based keying when unauthenticated.
func UserRateLimit(requestLimit int, windowLength time.Duration) func(http.Handler) http.Handler {
	return httprate.Limit(requestLimit, windowLength,
		httprate.WithKeyFuncs(KeyByUserID),
		httprate.WithLimitHandler(rateLimitHandler),
	)
}

// rateLimitHandler is the custom 429 response matching our existing JSON format.
func rateLimitHandler(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
}
