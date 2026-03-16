package shared

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Use applies middleware to a handler, wrapping in right-to-left order.
func Use(h http.HandlerFunc, mws ...func(http.Handler) http.Handler) http.Handler {
	var handler http.Handler = h
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler
}

// CORSMiddleware handles CORS for the API.
// Supports exact origins and wildcard subdomain patterns like "https://*.forumline.net".
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS") // comma-separated
		if allowedOrigins == "" {
			allowedOrigins = "https://app.forumline.net"
		}

		allowed := false
		for _, o := range strings.Split(allowedOrigins, ",") {
			o = strings.TrimSpace(o)
			if o == origin {
				allowed = true
				break
			}
			// Support wildcard subdomain patterns like "https://*.forumline.net"
			if strings.HasPrefix(o, "https://*.") {
				suffix := o[len("https://*"):]  // e.g. ".forumline.net"
				if strings.HasPrefix(origin, "https://") && strings.HasSuffix(origin, suffix) {
					// Ensure it's a direct subdomain, not nested
					host := origin[len("https://"):]
					host = strings.TrimSuffix(host, suffix)
					if host != "" && !strings.Contains(host, ".") {
						allowed = true
						break
					}
				}
			}
		}

		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Forumline-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders adds standard security headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}

// RateLimitResult contains the outcome of a rate limit check, including
// data needed for standard X-RateLimit-* response headers.
type RateLimitResult struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
}

// Limiter is the interface for rate limiting implementations.
// Both in-memory and Valkey-backed limiters satisfy this.
type Limiter interface {
	Allow(key string) bool
	Check(key string) RateLimitResult
	Stop()
}

// RateLimiter provides simple in-memory per-IP rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
	cancel   context.CancelFunc
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	rl := &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
		cancel:   cancel,
	}
	// Cleanup old entries periodically
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.cleanup()
			case <-ctx.Done():
				return
			}
		}
	}()
	return rl
}

// Stop cancels the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	rl.cancel()
}

func (rl *RateLimiter) Allow(key string) bool {
	return rl.Check(key).Allowed
}

func (rl *RateLimiter) Check(key string) RateLimitResult {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Filter old requests
	reqs := rl.requests[key]
	filtered := reqs[:0]
	for _, t := range reqs {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) >= rl.limit {
		rl.requests[key] = filtered
		// Reset is when the oldest request in the window expires
		resetAt := now.Add(rl.window)
		if len(filtered) > 0 {
			resetAt = filtered[0].Add(rl.window)
		}
		return RateLimitResult{Allowed: false, Limit: rl.limit, Remaining: 0, ResetAt: resetAt}
	}

	rl.requests[key] = append(filtered, now)
	remaining := rl.limit - len(filtered) - 1
	return RateLimitResult{Allowed: true, Limit: rl.limit, Remaining: remaining, ResetAt: now.Add(rl.window)}
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	for ip, reqs := range rl.requests {
		filtered := reqs[:0]
		for _, t := range reqs {
			if t.After(cutoff) {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = filtered
		}
	}
}

// ValkeyRateLimiter uses Valkey INCR + EXPIRE for distributed rate limiting.
// Falls back to an in-memory limiter if Valkey is unavailable.
type ValkeyRateLimiter struct {
	client   *redis.Client
	limit    int
	window   time.Duration
	fallback *RateLimiter
}

// NewValkeyRateLimiter creates a Valkey-backed rate limiter with in-memory fallback.
// If client is nil, all calls delegate to the fallback immediately.
func NewValkeyRateLimiter(client *redis.Client, limit int, window time.Duration) *ValkeyRateLimiter {
	return &ValkeyRateLimiter{
		client:   client,
		limit:    limit,
		window:   window,
		fallback: NewRateLimiter(limit, window),
	}
}

func (vrl *ValkeyRateLimiter) Allow(key string) bool {
	return vrl.Check(key).Allowed
}

func (vrl *ValkeyRateLimiter) Check(key string) RateLimitResult {
	if vrl.client == nil {
		return vrl.fallback.Check(key)
	}

	ctx := context.Background()
	rkey := ValkeyKey("rl", key)

	count, err := vrl.client.Incr(ctx, rkey).Result()
	if err != nil {
		return vrl.fallback.Check(key)
	}

	// Set expiry only on first increment (count == 1)
	if count == 1 {
		vrl.client.Expire(ctx, rkey, vrl.window)
	}

	// Get TTL for reset time
	resetAt := time.Now().Add(vrl.window)
	if ttl, err := vrl.client.TTL(ctx, rkey).Result(); err == nil && ttl > 0 {
		resetAt = time.Now().Add(ttl)
	}

	allowed := int(count) <= vrl.limit
	remaining := vrl.limit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	return RateLimitResult{Allowed: allowed, Limit: vrl.limit, Remaining: remaining, ResetAt: resetAt}
}

func (vrl *ValkeyRateLimiter) Stop() {
	vrl.fallback.Stop()
}

// clientIP extracts the IP address from RemoteAddr, stripping the port.
func clientIP(remoteAddr string) string {
	// RemoteAddr is "host:port" — strip the port
	if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
		return remoteAddr[:idx]
	}
	return remoteAddr
}

// setRateLimitHeaders writes standard X-RateLimit-* headers to the response.
func setRateLimitHeaders(w http.ResponseWriter, result RateLimitResult) {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", result.Limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", result.Remaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", result.ResetAt.Unix()))
}

// requestIP extracts the client IP from the request, respecting X-Forwarded-For
// when TRUST_PROXY is set. Always strips the port from RemoteAddr.
func requestIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			return strings.TrimSpace(strings.Split(forwarded, ",")[0])
		}
	}
	return clientIP(r.RemoteAddr)
}

// RateLimitMiddleware applies rate limiting per IP with standard X-RateLimit-* headers.
// Set TRUST_PROXY=true when running behind a known reverse proxy to use X-Forwarded-For.
func RateLimitMiddleware(rl Limiter) func(http.Handler) http.Handler {
	trustProxy := os.Getenv("TRUST_PROXY") == "true"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := requestIP(r, trustProxy)

			result := rl.Check(ip)
			setRateLimitHeaders(w, result)

			if !result.Allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(time.Until(result.ResetAt).Seconds())+1))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// UserRateLimitMiddleware applies rate limiting per authenticated user ID.
// Uses the user ID from the JWT context (set by AuthMiddleware). Falls back
// to IP-based limiting if no user ID is present.
func UserRateLimitMiddleware(rl Limiter) func(http.Handler) http.Handler {
	trustProxy := os.Getenv("TRUST_PROXY") == "true"
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := UserIDFromContext(r.Context())
			if key == "" {
				key = requestIP(r, trustProxy)
			}

			result := rl.Check(key)
			setRateLimitHeaders(w, result)

			if !result.Allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", int(time.Until(result.ResetAt).Seconds())+1))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
