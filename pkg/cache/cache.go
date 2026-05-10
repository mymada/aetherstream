package cache

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// Cache is a generic key-value cache with TTL support.
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
}

// entry holds a cached value with its expiration time.
type entry struct {
	key       string
	value     interface{}
	expiresAt time.Time
}

// LRUCache is a thread-safe in-memory LRU cache with TTL.
type LRUCache struct {
	mu       sync.Mutex
	items    map[string]*list.Element
	order    *list.List
	maxSize  int
}

// NewLRUCache creates an LRU cache with the given maximum number of entries.
func NewLRUCache(maxSize int) *LRUCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &LRUCache{
		items:   make(map[string]*list.Element),
		order:   list.New(),
		maxSize: maxSize,
	}
}

// Get retrieves a value from the cache. Returns false if the key is missing
// or the entry has expired.
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return nil, false
	}

	e := el.Value.(*entry)
	if time.Now().After(e.expiresAt) {
		c.removeElement(el)
		return nil, false
	}

	c.order.MoveToFront(el)
	return e.value, true
}

// Set stores a value in the cache with the specified TTL.
func (c *LRUCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*entry).value = value
		el.Value.(*entry).expiresAt = time.Now().Add(ttl)
		return
	}

	e := &entry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	el := c.order.PushFront(e)
	c.items[key] = el

	if c.order.Len() > c.maxSize {
		c.evictOldest()
	}
}

// Delete removes a key from the cache.
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.removeElement(el)
	}
}

// Clear removes all entries from the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.order.Init()
}

// removeElement removes an element from both the list and the map.
func (c *LRUCache) removeElement(el *list.Element) {
	c.order.Remove(el)
	e := el.Value.(*entry)
	delete(c.items, e.key)
}

// evictOldest removes the least recently used entry.
func (c *LRUCache) evictOldest() {
	el := c.order.Back()
	if el != nil {
		c.removeElement(el)
	}
}

// --- Key helpers for AetherStream ---

// MetadataKey returns a cache key for metadata lookups.
func MetadataKey(kind, id string) string {
	return fmt.Sprintf("metadata:%s:%s", kind, id)
}

// PosterKey returns a cache key for poster URLs.
func PosterKey(itemID string) string {
	return fmt.Sprintf("poster:%s", itemID)
}

// EPGKey returns a cache key for EPG data.
func EPGKey(channelID string) string {
	return fmt.Sprintf("epg:%s", channelID)
}
