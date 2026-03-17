package platform

import (
	"container/list"
	"testing"
	"time"
)

func TestSiteCache_PutAndGet(t *testing.T) {
	c := NewSiteCache(1, 5*time.Minute)
	c.Put("forum1", "index.html", []byte("<html>"), "text/html", "etag1")

	data, ct, etag, ok := c.Get("forum1", "index.html")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(data) != "<html>" {
		t.Errorf("data = %q", data)
	}
	if ct != "text/html" {
		t.Errorf("content-type = %q", ct)
	}
	if etag != "etag1" {
		t.Errorf("etag = %q", etag)
	}
}

func TestSiteCache_Miss(t *testing.T) {
	c := NewSiteCache(1, 5*time.Minute)
	_, _, _, ok := c.Get("nonexistent", "file.js")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestSiteCache_TTLExpiry(t *testing.T) {
	c := NewSiteCache(1, 1*time.Millisecond)
	c.Put("slug", "file.css", []byte("body{}"), "text/css", "e1")

	time.Sleep(5 * time.Millisecond)

	_, _, _, ok := c.Get("slug", "file.css")
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
}

func TestSiteCache_LRUEviction(t *testing.T) {
	// 1KB max cache — use the struct directly for precise control
	c := &SiteCache{
		items:   make(map[string]*list.Element),
		order:   list.New(),
		maxSize: 1024,
		ttl:     5 * time.Minute,
	}

	// Fill with 512 bytes
	c.Put("a", "f1", make([]byte, 512), "app/octet-stream", "e1")
	// Fill remaining 512 bytes
	c.Put("a", "f2", make([]byte, 512), "app/octet-stream", "e2")

	// Access f1 to make it most recently used
	c.Get("a", "f1")

	// Add 512 more bytes — should evict f2 (LRU)
	c.Put("a", "f3", make([]byte, 512), "app/octet-stream", "e3")

	_, _, _, ok := c.Get("a", "f2")
	if ok {
		t.Error("f2 should have been evicted (LRU)")
	}

	_, _, _, ok = c.Get("a", "f1")
	if !ok {
		t.Error("f1 should still be in cache (recently accessed)")
	}
}

func TestSiteCache_RejectsLargeFiles(t *testing.T) {
	c := NewSiteCache(10, 5*time.Minute) // 10MB cache
	bigData := make([]byte, (1<<20)+1)   // just over 1MB

	c.Put("slug", "big.bin", bigData, "application/octet-stream", "e1")

	_, _, _, ok := c.Get("slug", "big.bin")
	if ok {
		t.Error("files over 1MB should not be cached")
	}
}

func TestSiteCache_OverwriteExisting(t *testing.T) {
	c := NewSiteCache(1, 5*time.Minute)
	c.Put("s", "f", []byte("v1"), "text/plain", "e1")
	c.Put("s", "f", []byte("v2"), "text/plain", "e2")

	data, _, etag, ok := c.Get("s", "f")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(data) != "v2" {
		t.Errorf("expected v2, got %q", data)
	}
	if etag != "e2" {
		t.Errorf("expected e2, got %q", etag)
	}
}

func TestSiteCache_Invalidate(t *testing.T) {
	c := NewSiteCache(1, 5*time.Minute)
	c.Put("s", "f", []byte("data"), "text/plain", "e1")

	c.Invalidate("s", "f")

	_, _, _, ok := c.Get("s", "f")
	if ok {
		t.Error("expected cache miss after invalidation")
	}
}

func TestSiteCache_Invalidate_NonExistent(t *testing.T) {
	c := NewSiteCache(1, 5*time.Minute)
	// Should not panic
	c.Invalidate("nope", "nope")
}

func TestCacheKey(t *testing.T) {
	got := cacheKey("myslug", "assets/main.js")
	if got != "myslug/assets/main.js" {
		t.Errorf("cacheKey = %q", got)
	}
}
