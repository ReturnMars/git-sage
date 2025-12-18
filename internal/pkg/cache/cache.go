// Package cache provides response caching for GitSage.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

const (
	// DefaultMaxEntries is the default maximum number of cache entries.
	DefaultMaxEntries = 100
	// DefaultTTL is the default time-to-live for cache entries.
	DefaultTTL = 1 * time.Hour
)

// Entry represents a cached response.
type Entry struct {
	Value     interface{}
	ExpiresAt time.Time
}

// IsExpired checks if the cache entry has expired.
func (e *Entry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Manager defines the interface for cache management.
type Manager interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
	Size() int
}

// LRUCache implements an in-memory LRU cache with TTL support.
type LRUCache struct {
	mu         sync.RWMutex
	entries    map[string]*Entry
	order      []string // Tracks access order for LRU eviction
	maxEntries int
	defaultTTL time.Duration
}

// NewLRUCache creates a new LRU cache with the specified configuration.
func NewLRUCache(maxEntries int, defaultTTL time.Duration) *LRUCache {
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}
	if defaultTTL <= 0 {
		defaultTTL = DefaultTTL
	}
	return &LRUCache{
		entries:    make(map[string]*Entry),
		order:      make([]string, 0, maxEntries),
		maxEntries: maxEntries,
		defaultTTL: defaultTTL,
	}
}

// Get retrieves a value from the cache.
// Returns the value and true if found and not expired, nil and false otherwise.
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if entry.IsExpired() {
		c.deleteUnlocked(key)
		return nil, false
	}

	// Move to end of order (most recently used)
	c.moveToEnd(key)

	return entry.Value, true
}

// Set stores a value in the cache with the specified TTL.
// If ttl is 0, the default TTL is used.
func (c *LRUCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	// Check if key already exists
	if _, exists := c.entries[key]; exists {
		// Update existing entry
		c.entries[key] = &Entry{
			Value:     value,
			ExpiresAt: time.Now().Add(ttl),
		}
		c.moveToEnd(key)
		return
	}

	// Evict oldest entry if at capacity
	if len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	// Add new entry
	c.entries[key] = &Entry{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
	c.order = append(c.order, key)
}

// Delete removes a value from the cache.
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteUnlocked(key)
}

// Clear removes all entries from the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*Entry)
	c.order = make([]string, 0, c.maxEntries)
}

// Size returns the number of entries in the cache.
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// deleteUnlocked removes an entry without acquiring the lock.
// Caller must hold the lock.
func (c *LRUCache) deleteUnlocked(key string) {
	delete(c.entries, key)
	c.removeFromOrder(key)
}

// evictOldest removes the oldest (least recently used) entry.
// Caller must hold the lock.
func (c *LRUCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}
	oldest := c.order[0]
	c.deleteUnlocked(oldest)
}

// moveToEnd moves a key to the end of the order slice (most recently used).
// Caller must hold the lock.
func (c *LRUCache) moveToEnd(key string) {
	c.removeFromOrder(key)
	c.order = append(c.order, key)
}

// removeFromOrder removes a key from the order slice.
// Caller must hold the lock.
func (c *LRUCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

// GenerateCacheKey generates a cache key from the given components.
// Uses SHA256 hash of: diff + provider + model + prompt
func GenerateCacheKey(diff, provider, model, prompt string) string {
	data := diff + "|" + provider + "|" + model + "|" + prompt
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// CleanExpired removes all expired entries from the cache.
func (c *LRUCache) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	for key, entry := range c.entries {
		if entry.IsExpired() {
			c.deleteUnlocked(key)
			removed++
		}
	}
	return removed
}
