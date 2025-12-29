package cache

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/gitsage/gitsage/internal/core/domain"
)

type entry struct {
	key       string
	value     *domain.CommitMessage
	expiresAt time.Time
}

// LRUCache implements a thread-safe LRU cache.
type LRUCache struct {
	capacity  int
	ttl       time.Duration
	mu        sync.Mutex
	items     map[string]*list.Element
	evictList *list.List
}

func NewLRUCache(capacity int, ttl time.Duration) *LRUCache {
	if capacity <= 0 {
		capacity = 100
	}
	return &LRUCache{
		capacity:  capacity,
		ttl:       ttl,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
	}
}

func (c *LRUCache) Set(key string, value *domain.CommitMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check for existing item
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		e := ent.Value.(*entry)
		e.value = value
		e.expiresAt = time.Now().Add(c.ttl)
		return
	}

	// Add new item
	ent := &entry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	element := c.evictList.PushFront(ent)
	c.items[key] = element

	// Evict if over capacity
	if c.evictList.Len() > c.capacity {
		c.removeOldest()
	}
}

func (c *LRUCache) Get(key string) (*domain.CommitMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ent, ok := c.items[key]; ok {
		e := ent.Value.(*entry)
		if time.Now().After(e.expiresAt) {
			c.removeElement(ent)
			return nil, false
		}
		c.evictList.MoveToFront(ent)
		return e.value, true
	}
	return nil, false
}

func (c *LRUCache) removeOldest() {
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
	}
}

func (c *LRUCache) removeElement(e *list.Element) {
	c.evictList.Remove(e)
	kv := e.Value.(*entry)
	delete(c.items, kv.key)
}

// GenerateKey generates a cache key from input components.
func GenerateKey(diffContent, hint string) string {
	hash := sha256.New()
	hash.Write([]byte(diffContent))
	hash.Write([]byte("|"))
	hash.Write([]byte(hint))
	return hex.EncodeToString(hash.Sum(nil))
}
