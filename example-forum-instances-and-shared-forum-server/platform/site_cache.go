package platform

import (
	"sync"
	"time"
)

// SiteCache is an in-memory LRU cache for custom site files served from R2.
// Keyed by "{slug}/{path}", with TTL-based expiration and a max total size.
type SiteCache struct {
	mu       sync.Mutex
	items    map[string]*cacheItem
	order    []string // LRU order (most recent at end)
	curSize  int64
	maxSize  int64
	ttl      time.Duration
}

type cacheItem struct {
	data        []byte
	contentType string
	etag        string
	added       time.Time
}

func NewSiteCache(maxSizeMB int, ttl time.Duration) *SiteCache {
	return &SiteCache{
		items:   make(map[string]*cacheItem),
		maxSize: int64(maxSizeMB) * 1024 * 1024,
		ttl:     ttl,
	}
}

func cacheKey(slug, path string) string {
	return slug + "/" + path
}

func (c *SiteCache) Get(slug, path string) (data []byte, contentType string, etag string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(slug, path)
	item, exists := c.items[key]
	if !exists {
		return nil, "", "", false
	}
	if time.Since(item.added) > c.ttl {
		c.removeLocked(key)
		return nil, "", "", false
	}
	// Move to end (most recently used)
	c.touchLocked(key)
	return item.data, item.contentType, item.etag, true
}

func (c *SiteCache) Put(slug, path string, data []byte, contentType, etag string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(slug, path)
	size := int64(len(data))

	// Don't cache files larger than 1MB individually
	if size > 1<<20 {
		return
	}

	// Remove old entry if exists
	if old, exists := c.items[key]; exists {
		c.curSize -= int64(len(old.data))
		c.removeLocked(key)
	}

	// Evict until we have space
	for c.curSize+size > c.maxSize && len(c.order) > 0 {
		c.removeLocked(c.order[0])
	}

	c.items[key] = &cacheItem{
		data:        data,
		contentType: contentType,
		etag:        etag,
		added:       time.Now(),
	}
	c.order = append(c.order, key)
	c.curSize += size
}

func (c *SiteCache) Invalidate(slug, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(cacheKey(slug, path))
}

func (c *SiteCache) removeLocked(key string) {
	if item, exists := c.items[key]; exists {
		c.curSize -= int64(len(item.data))
		delete(c.items, key)
	}
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

func (c *SiteCache) touchLocked(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			break
		}
	}
}
