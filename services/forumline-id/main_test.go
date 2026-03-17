package main

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/forumline/forumline/services/forumline-id/oapi"
)

// --- codeStore ---

func TestCodeStore_StoreAndConsume(t *testing.T) {
	cs := &codeStore{codes: make(map[string]*authCode)}

	ui := &oapi.UserInfo{ForumlineId: "user-1", Username: "alice"}
	cs.Store("abc123", &authCode{
		UserInfo:    ui,
		RedirectURI: "https://test.forumline.net/callback",
		ExpiresAt:   time.Now().Add(60 * time.Second),
	})

	ac, ok := cs.Consume("abc123", "https://test.forumline.net/callback")
	if !ok {
		t.Fatal("expected successful consume")
	}
	if ac.UserInfo.ForumlineId != "user-1" {
		t.Errorf("ForumlineId = %q", ac.UserInfo.ForumlineId)
	}
}

func TestCodeStore_SingleUse(t *testing.T) {
	cs := &codeStore{codes: make(map[string]*authCode)}
	cs.Store("code1", &authCode{
		UserInfo:    &oapi.UserInfo{},
		RedirectURI: "https://a.forumline.net/cb",
		ExpiresAt:   time.Now().Add(60 * time.Second),
	})

	_, ok := cs.Consume("code1", "https://a.forumline.net/cb")
	if !ok {
		t.Fatal("first consume should succeed")
	}

	_, ok = cs.Consume("code1", "https://a.forumline.net/cb")
	if ok {
		t.Error("second consume should fail — code is single-use")
	}
}

func TestCodeStore_ExpiredCode(t *testing.T) {
	cs := &codeStore{codes: make(map[string]*authCode)}
	cs.Store("expired", &authCode{
		UserInfo:    &oapi.UserInfo{},
		RedirectURI: "https://a.forumline.net/cb",
		ExpiresAt:   time.Now().Add(-1 * time.Second), // already expired
	})

	_, ok := cs.Consume("expired", "https://a.forumline.net/cb")
	if ok {
		t.Error("expired code should not be consumable")
	}
}

func TestCodeStore_WrongRedirectURI(t *testing.T) {
	cs := &codeStore{codes: make(map[string]*authCode)}
	cs.Store("code2", &authCode{
		UserInfo:    &oapi.UserInfo{},
		RedirectURI: "https://legit.forumline.net/cb",
		ExpiresAt:   time.Now().Add(60 * time.Second),
	})

	_, ok := cs.Consume("code2", "https://evil.example.com/steal")
	if ok {
		t.Error("consume with wrong redirect_uri should fail")
	}

	// Code must be invalidated after any failed attempt (RFC 6749 §4.1.2)
	_, ok = cs.Consume("code2", "https://legit.forumline.net/cb")
	if ok {
		t.Error("code should be gone after failed redirect_uri check — single-use means ANY attempt")
	}
}

func TestCodeStore_NonExistentCode(t *testing.T) {
	cs := &codeStore{codes: make(map[string]*authCode)}
	_, ok := cs.Consume("doesnt-exist", "https://a.forumline.net/cb")
	if ok {
		t.Error("non-existent code should return false")
	}
}

// --- isAllowedRedirect ---

func TestIsAllowedRedirect(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		allowed bool
	}{
		{"forumline.net", "https://forumline.net/cb", true},
		{"subdomain", "https://test.forumline.net/callback", true},
		{"deep subdomain", "https://a.b.forumline.net/cb", true},
		{"localhost", "http://localhost:3000/cb", true},
		{"127.0.0.1", "http://127.0.0.1:5173/cb", true},
		{"evil domain", "https://evil.com/cb", false},
		{"forumline suffix attack", "https://notforumline.net/cb", false},
		{"forumline in path", "https://evil.com/forumline.net", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.rawURL)
			if err != nil {
				t.Fatalf("bad test URL: %v", err)
			}
			got := isAllowedRedirect(u)
			if got != tt.allowed {
				t.Errorf("isAllowedRedirect(%q) = %v, want %v", tt.rawURL, got, tt.allowed)
			}
		})
	}
}

// --- extractBearerToken ---

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		query  string
		want   string
	}{
		{"from header", "Bearer my-jwt-token", "", "my-jwt-token"},
		{"from query", "", "my-query-token", "my-query-token"},
		{"header takes precedence", "Bearer header-token", "query-token", "header-token"},
		{"no token", "", "", ""},
		{"wrong prefix", "Basic abc123", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqURL := "http://localhost/userinfo"
			if tt.query != "" {
				reqURL += "?access_token=" + tt.query
			}
			req, err := http.NewRequest("GET", reqURL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			got := extractBearerToken(req)
			if got != tt.want {
				t.Errorf("extractBearerToken = %q, want %q", got, tt.want)
			}
		})
	}
}
