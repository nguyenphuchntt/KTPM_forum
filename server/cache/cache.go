package cache

import (
	"sync"
	"time"
)

type Cache struct {
	mu    sync.RWMutex
	items map[string]CacheItem
}

func New() *Cache {
	return &Cache{
		items: make(map[string]CacheItem),
	}
}

// SET
func (cache *Cache) Set(key string, data interface{}, ttl time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	var ex int64
	if ttl > 0 {
		ex = time.Now().Add(ttl).UnixNano()
	}

	cache.items[key] = CacheItem{
		Data:      data,
		ExpiresAt: ex,
	}
}

// GET
func (cache *Cache) Get(key string) (interface{}, bool) {
	cache.mu.RLock()
	item, found := cache.items[key]
	cache.mu.RUnlock()

	if !found {
		return nil, false
	}

	if IsExpired(item) {
		cache.Delete(key)
		return nil, false
	}

	return item.Data, true
}

// DELETE
func (cache *Cache) Delete(key string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	delete(cache.items, key)
}

var AppCache = New()
