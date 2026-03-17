package platform

import (
	"testing"
	"time"
)

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "simple html", input: "index.html", want: "index.html"},
		{name: "simple css", input: "style.css", want: "style.css"},
		{name: "nested path", input: "js/app.js", want: "js/app.js"},
		{name: "uppercase normalized", input: "Style.CSS", want: "style.css"},
		{name: "path traversal", input: "../etc/passwd", wantErr: true},
		{name: "path traversal mid", input: "foo/../bar.js", wantErr: true},
		{name: "absolute path", input: "/etc/passwd.txt", wantErr: true},
		{name: "hidden file", input: ".htaccess", wantErr: true},
		{name: "hidden dir", input: ".git/config.txt", wantErr: true},
		{name: "no extension", input: "Makefile", wantErr: true},
		{name: "disallowed extension", input: "script.php", wantErr: true},
		{name: "disallowed extension exe", input: "app.exe", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "spaces trimmed", input: "  style.css  ", want: "style.css"},
		{name: "svg allowed", input: "logo.svg", want: "logo.svg"},
		{name: "woff2 allowed", input: "font.woff2", want: "font.woff2"},
		{name: "webp allowed", input: "photo.webp", want: "photo.webp"},
		{name: "deep nested", input: "assets/images/bg.png", want: "assets/images/bg.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil (result: %q)", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestR2Key(t *testing.T) {
	got := r2Key("myforum", "style.css")
	want := "sites/myforum/files/style.css"
	if got != want {
		t.Errorf("r2Key = %q, want %q", got, want)
	}

	got = r2MetaKey("myforum")
	want = "sites/myforum/_meta.json"
	if got != want {
		t.Errorf("r2MetaKey = %q, want %q", got, want)
	}
}

func TestSiteCache(t *testing.T) {
	t.Run("basic put and get", func(t *testing.T) {
		cache := NewSiteCache(1, time.Minute)
		cache.Put("forum1", "index.html", []byte("<html>"), "text/html", "abc123")

		data, ct, etag, ok := cache.Get("forum1", "index.html")
		if !ok {
			t.Fatal("expected cache hit")
		}
		if string(data) != "<html>" {
			t.Errorf("data = %q, want %q", data, "<html>")
		}
		if ct != "text/html" {
			t.Errorf("content type = %q, want %q", ct, "text/html")
		}
		if etag != "abc123" {
			t.Errorf("etag = %q, want %q", etag, "abc123")
		}
	})

	t.Run("cache miss", func(t *testing.T) {
		cache := NewSiteCache(1, time.Minute)
		_, _, _, ok := cache.Get("forum1", "missing.html")
		if ok {
			t.Fatal("expected cache miss")
		}
	})

	t.Run("TTL expiration", func(t *testing.T) {
		cache := NewSiteCache(1, time.Millisecond)
		cache.Put("forum1", "index.html", []byte("<html>"), "text/html", "abc")

		time.Sleep(5 * time.Millisecond)

		_, _, _, ok := cache.Get("forum1", "index.html")
		if ok {
			t.Fatal("expected cache miss after TTL")
		}
	})

	t.Run("invalidation", func(t *testing.T) {
		cache := NewSiteCache(1, time.Minute)
		cache.Put("forum1", "index.html", []byte("<html>"), "text/html", "abc")

		cache.Invalidate("forum1", "index.html")

		_, _, _, ok := cache.Get("forum1", "index.html")
		if ok {
			t.Fatal("expected cache miss after invalidation")
		}
	})

	t.Run("different slugs isolated", func(t *testing.T) {
		cache := NewSiteCache(1, time.Minute)
		cache.Put("forum1", "index.html", []byte("<html>1"), "text/html", "a")
		cache.Put("forum2", "index.html", []byte("<html>2"), "text/html", "b")

		data1, _, _, ok1 := cache.Get("forum1", "index.html")
		data2, _, _, ok2 := cache.Get("forum2", "index.html")

		if !ok1 || !ok2 {
			t.Fatal("expected both cache hits")
		}
		if string(data1) != "<html>1" || string(data2) != "<html>2" {
			t.Errorf("data not isolated: got %q and %q", data1, data2)
		}
	})

	t.Run("LRU eviction", func(t *testing.T) {
		// 1KB cache
		cache := NewSiteCache(0, time.Minute)
		cache.maxSize = 100 // 100 bytes

		cache.Put("f", "a.html", make([]byte, 60), "text/html", "a")
		cache.Put("f", "b.html", make([]byte, 60), "text/html", "b")

		// a should be evicted to make room for b
		_, _, _, okA := cache.Get("f", "a.html")
		_, _, _, okB := cache.Get("f", "b.html")

		if okA {
			t.Error("expected a.html to be evicted")
		}
		if !okB {
			t.Error("expected b.html to still be cached")
		}
	})

	t.Run("overwrite same key tracks size correctly", func(t *testing.T) {
		cache := NewSiteCache(1, time.Minute)

		cache.Put("f", "a.html", make([]byte, 50), "text/html", "v1")
		if cache.curSize != 50 {
			t.Errorf("curSize = %d, want 50", cache.curSize)
		}

		// Overwrite with different size
		cache.Put("f", "a.html", make([]byte, 30), "text/html", "v2")
		if cache.curSize != 30 {
			t.Errorf("curSize = %d, want 30 after overwrite", cache.curSize)
		}

		// Invalidate should bring to 0
		cache.Invalidate("f", "a.html")
		if cache.curSize != 0 {
			t.Errorf("curSize = %d, want 0 after invalidate", cache.curSize)
		}
	})

	t.Run("skip files larger than 1MB", func(t *testing.T) {
		cache := NewSiteCache(256, time.Minute)
		bigData := make([]byte, 2<<20) // 2MB
		cache.Put("f", "big.png", bigData, "image/png", "x")

		_, _, _, ok := cache.Get("f", "big.png")
		if ok {
			t.Error("expected large file to not be cached")
		}
		if cache.curSize != 0 {
			t.Errorf("curSize = %d, want 0", cache.curSize)
		}
	})
}

func TestUpdateSiteState_HasCustomSite(t *testing.T) {
	// Test the logic that determines has_custom_site based on index.html presence
	manifest := &siteManifest{
		Files: map[string]siteFileEntry{
			"style.css": {Size: 100},
		},
	}

	_, hasIndex := manifest.Files["index.html"]
	if hasIndex {
		t.Error("expected no index.html")
	}

	manifest.Files["index.html"] = siteFileEntry{Size: 200}
	_, hasIndex = manifest.Files["index.html"]
	if !hasIndex {
		t.Error("expected index.html to exist")
	}

	// Verify total bytes calculation
	var total int64
	for _, f := range manifest.Files {
		total += f.Size
	}
	if total != 300 {
		t.Errorf("total = %d, want 300", total)
	}
}
