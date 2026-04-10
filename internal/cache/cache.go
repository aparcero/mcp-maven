package cache

import (
	"sync"
	"time"
)

// Cache is a simple TTL-based in-memory cache.
type Cache struct {
	mu      sync.RWMutex
	items   map[string]*cacheItem
	order   []string
	maxSize int
}

// cacheItem represents a single cache entry.
type cacheItem struct {
	value      any
	expiration time.Time
}

// New creates a new cache.
func New(maxSize ...int) *Cache {
	limit := 0
	if len(maxSize) > 0 && maxSize[0] > 0 {
		limit = maxSize[0]
	}
	c := &Cache{
		items:   make(map[string]*cacheItem),
		order:   []string{},
		maxSize: limit,
	}
	// Start cleanup goroutine
	go c.cleanupLoop()
	return c
}

// Get retrieves a value from the cache.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[key]
	if !found {
		return nil, false
	}

	if time.Now().After(item.expiration) {
		return nil, false
	}

	return item.value, true
}

// Set stores a value in the cache with the specified TTL.
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	c.cleanupExpiredLocked(now)
	if _, exists := c.items[key]; !exists {
		c.order = append(c.order, key)
	}
	c.items[key] = &cacheItem{
		value:      value,
		expiration: now.Add(ttl),
	}
	c.enforceMaxSize()
}

// Delete removes a value from the cache.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.deleteLocked(key)
}

// Clear removes all items from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem)
	c.order = []string{}
}

// cleanupLoop periodically removes expired items.
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired items.
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanupExpiredLocked(time.Now())
}

func (c *Cache) cleanupExpiredLocked(now time.Time) {
	for key, item := range c.items {
		if now.After(item.expiration) {
			c.deleteLocked(key)
		}
	}
}

// Size returns the number of items in the cache.
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Filter out expired items
	now := time.Now()
	count := 0
	for _, item := range c.items {
		if now.Before(item.expiration) {
			count++
		}
	}
	return count
}

func (c *Cache) enforceMaxSize() {
	if c.maxSize <= 0 {
		return
	}
	for len(c.items) > c.maxSize && len(c.order) > 0 {
		c.deleteLocked(c.order[0])
	}
}

func (c *Cache) deleteLocked(key string) {
	delete(c.items, key)
	for i, orderedKey := range c.order {
		if orderedKey == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}
