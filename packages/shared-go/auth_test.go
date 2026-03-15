package shared

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()
	if got := UserIDFromContext(ctx); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestUserIDFromContext_WithValue(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, "test-user")
	if got := UserIDFromContext(ctx); got != "test-user" {
		t.Errorf("got %q, want %q", got, "test-user")
	}
}

func TestPromoteQueryToken_NoHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?access_token=my-token", nil)
	got := promoteQueryToken(req)
	if auth := got.Header.Get("Authorization"); auth != "Bearer my-token" {
		t.Errorf("Authorization = %q, want %q", auth, "Bearer my-token")
	}
}

func TestPromoteQueryToken_HeaderTakesPriority(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?access_token=query-token", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	got := promoteQueryToken(req)
	if auth := got.Header.Get("Authorization"); auth != "Bearer header-token" {
		t.Errorf("Authorization = %q, want %q (header should take priority)", auth, "Bearer header-token")
	}
}

func TestPromoteQueryToken_NoToken(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	got := promoteQueryToken(req)
	if auth := got.Header.Get("Authorization"); auth != "" {
		t.Errorf("Authorization = %q, want empty", auth)
	}
}

func TestExtractToken_BearerHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer my-token")
	if got := extractToken(req); got != "my-token" {
		t.Errorf("extractToken = %q, want %q", got, "my-token")
	}
}

func TestExtractToken_QueryParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?access_token=query-token", nil)
	if got := extractToken(req); got != "query-token" {
		t.Errorf("extractToken = %q, want %q", got, "query-token")
	}
}

func TestExtractToken_HeaderPriority(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?access_token=query-token", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	if got := extractToken(req); got != "header-token" {
		t.Errorf("extractToken = %q, want %q", got, "header-token")
	}
}

func TestExtractToken_Empty(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	if got := extractToken(req); got != "" {
		t.Errorf("extractToken = %q, want empty", got)
	}
}

func TestInitAuth_MissingURL(t *testing.T) {
	t.Setenv("ZITADEL_URL", "")
	t.Setenv("ZITADEL_CLIENT_ID", "test-client")
	if err := InitAuth(context.Background()); err == nil {
		t.Fatal("expected error when ZITADEL_URL is empty")
	}
}

func TestInitAuth_MissingClientID(t *testing.T) {
	t.Setenv("ZITADEL_URL", "https://auth.example.com")
	t.Setenv("ZITADEL_CLIENT_ID", "")
	if err := InitAuth(context.Background()); err == nil {
		t.Fatal("expected error when ZITADEL_CLIENT_ID is empty")
	}
}

// TestAuthMiddleware_Nil tests that AuthMiddleware panics or errors gracefully
// if InitAuth hasn't been called. In production, MustInitAuth is called at startup.
func TestAuthMiddleware_BeforeInit(t *testing.T) {
	// Reset global state
	old := zitadelMW
	zitadelMW = nil
	defer func() { zitadelMW = old }()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when zitadelMW is nil")
		}
	}()

	AuthMiddleware(handler).ServeHTTP(rr, req)
}
