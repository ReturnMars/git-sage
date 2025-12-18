package cache

import (
	"testing"
	"time"
)

func TestLRUCache_SetAndGet(t *testing.T) {
	cache := NewLRUCache(10, time.Hour)

	// Test basic set and get
	cache.Set("key1", "value1", 0)
	val, ok := cache.Get("key1")
	if !ok {
		t.Error("expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}

	// Test non-existent key
	_, ok = cache.Get("nonexistent")
	if ok {
		t.Error("expected nonexistent key to not exist")
	}
}

func TestLRUCache_Expiration(t *testing.T) {
	cache := NewLRUCache(10, 50*time.Millisecond)

	cache.Set("key1", "value1", 50*time.Millisecond)

	// Should exist immediately
	_, ok := cache.Get("key1")
	if !ok {
		t.Error("expected key1 to exist immediately")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok = cache.Get("key1")
	if ok {
		t.Error("expected key1 to be expired")
	}
}

func TestLRUCache_LRUEviction(t *testing.T) {
	cache := NewLRUCache(3, time.Hour)

	// Fill cache
	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)
	cache.Set("key3", "value3", 0)

	// Access key1 to make it recently used
	cache.Get("key1")

	// Add new entry, should evict key2 (oldest)
	cache.Set("key4", "value4", 0)

	// key2 should be evicted
	_, ok := cache.Get("key2")
	if ok {
		t.Error("expected key2 to be evicted")
	}

	// key1 should still exist (was accessed)
	_, ok = cache.Get("key1")
	if !ok {
		t.Error("expected key1 to still exist")
	}

	// key3 and key4 should exist
	_, ok = cache.Get("key3")
	if !ok {
		t.Error("expected key3 to exist")
	}
	_, ok = cache.Get("key4")
	if !ok {
		t.Error("expected key4 to exist")
	}
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(10, time.Hour)

	cache.Set("key1", "value1", 0)
	cache.Delete("key1")

	_, ok := cache.Get("key1")
	if ok {
		t.Error("expected key1 to be deleted")
	}
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(10, time.Hour)

	cache.Set("key1", "value1", 0)
	cache.Set("key2", "value2", 0)
	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}
}

func TestLRUCache_Size(t *testing.T) {
	cache := NewLRUCache(10, time.Hour)

	if cache.Size() != 0 {
		t.Errorf("expected size 0, got %d", cache.Size())
	}

	cache.Set("key1", "value1", 0)
	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}

	cache.Set("key2", "value2", 0)
	if cache.Size() != 2 {
		t.Errorf("expected size 2, got %d", cache.Size())
	}
}

func TestGenerateCacheKey(t *testing.T) {
	key1 := GenerateCacheKey("diff1", "openai", "gpt-4", "prompt1")
	key2 := GenerateCacheKey("diff1", "openai", "gpt-4", "prompt1")
	key3 := GenerateCacheKey("diff2", "openai", "gpt-4", "prompt1")

	// Same inputs should produce same key
	if key1 != key2 {
		t.Error("expected same inputs to produce same key")
	}

	// Different inputs should produce different key
	if key1 == key3 {
		t.Error("expected different inputs to produce different key")
	}

	// Key should be hex string of SHA256 (64 chars)
	if len(key1) != 64 {
		t.Errorf("expected key length 64, got %d", len(key1))
	}
}

func TestLRUCache_UpdateExisting(t *testing.T) {
	cache := NewLRUCache(10, time.Hour)

	cache.Set("key1", "value1", 0)
	cache.Set("key1", "value2", 0)

	val, ok := cache.Get("key1")
	if !ok {
		t.Error("expected key1 to exist")
	}
	if val != "value2" {
		t.Errorf("expected value2, got %v", val)
	}

	// Size should still be 1
	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}
}

func TestLRUCache_CleanExpired(t *testing.T) {
	cache := NewLRUCache(10, time.Hour)

	cache.Set("key1", "value1", 50*time.Millisecond)
	cache.Set("key2", "value2", time.Hour)

	// Wait for key1 to expire
	time.Sleep(100 * time.Millisecond)

	removed := cache.CleanExpired()
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	if cache.Size() != 1 {
		t.Errorf("expected size 1, got %d", cache.Size())
	}

	_, ok := cache.Get("key2")
	if !ok {
		t.Error("expected key2 to still exist")
	}
}
