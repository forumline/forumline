package service

import (
	"testing"
)

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{"valid domain", "example.com", false},
		{"valid subdomain", "forum.example.com", false},
		{"valid with port", "example.com:8080", false},
		{"empty", "", true},
		{"has path separator", "example.com/path", true},
		{"has query string", "example.com?q=1", true},
		{"has hash", "example.com#section", true},
		{"has at sign", "user@example.com", true},
		{"has space", "example .com", true},
		{"has tab", "example\t.com", true},
		{"has newline", "example\n.com", true},
		{"has carriage return", "example\r.com", true},
		{"ip address v4", "192.168.1.1", true},
		{"ip address v6", "::1", true},
		{"ip with port", "192.168.1.1:3000", true},
		{"single label no dot", "localhost", true},
		{"forumline.net", "forumline.net", false},
		{"deeply nested subdomain", "a.b.c.d.example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomain(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDomain(%q) error = %v, wantErr %v", tt.domain, err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name string
		raw  []string
		want int    // expected length
		has  string // one expected tag
	}{
		{"nil input", nil, 0, ""},
		{"empty slice", []string{}, 0, ""},
		{"lowercases", []string{"GAMING"}, 1, "gaming"},
		{"trims whitespace", []string{"  tech  "}, 1, "tech"},
		{"deduplicates", []string{"go", "Go", "GO"}, 1, "go"},
		{"empty after trim", []string{"  ", ""}, 0, ""},
		{"max 10 tags", []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}, 10, "a"},
		{"truncates at 32 runes", []string{"abcdefghijklmnopqrstuvwxyz1234567890"}, 1, "abcdefghijklmnopqrstuvwxyz123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeTags(tt.raw)
			if got == nil {
				t.Fatal("NormalizeTags returned nil, should return empty slice")
			}
			if len(got) != tt.want {
				t.Errorf("len = %d, want %d (tags: %v)", len(got), tt.want, got)
			}
			if tt.has != "" {
				found := false
				for _, tag := range got {
					if tag == tt.has {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected tag %q in %v", tt.has, got)
				}
			}
		})
	}
}

func TestNormalizeTags_PreservesOrder(t *testing.T) {
	got := NormalizeTags([]string{"zebra", "apple", "mango"})
	if len(got) != 3 || got[0] != "zebra" || got[1] != "apple" || got[2] != "mango" {
		t.Errorf("order not preserved: %v", got)
	}
}
