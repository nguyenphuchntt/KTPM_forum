package cache

import (
	"forum/server/config"
	"log"
	"time"
	"forum/server/metrics"


	lru "github.com/hashicorp/golang-lru/v2"
)

type Cache struct {
	lru *lru.Cache[string, CacheItem]
}

func New() *Cache {
	lruCache, err := lru.New[string, CacheItem](config.CacheSize)

	if err != nil {
		log.Fatalf("Failed to create LRU cache: %v", err)
	}
	return &Cache{
		lru: lruCache,
	}
}

// SET
func (cache *Cache) Set(key string, data interface{}, ttl time.Duration) {
	var ex time.Time
	if ttl > 0 {
		ex = time.Now().Add(ttl)
	}

	item := CacheItem{
		Data:      data,
		ExpiresAt: ex,
	}

	cache.lru.Add(key, item)
	metrics.CacheSize.Set(float64(cache.lru.Len()))
}

// GET
func (cache *Cache) Get(key string) (interface{}, bool) {
	item, found := cache.lru.Get(key)

	if !found {
		metrics.CacheMissesTotal.Inc()
		return nil, false
	}

	if IsExpired(item) {
		cache.Delete(key)
		metrics.CacheMissesTotal.Inc()
		return nil, false
	}

	metrics.CacheHitsTotal.Inc()
	return item.Data, true
}

// DELETE
func (cache *Cache) Delete(key string) {
	log.Println("Deleting cache key:", key)
	cache.lru.Remove(key)
	metrics.CacheSize.Set(float64(cache.lru.Len()))
}

var AppCache = New()

// Global category cache instance
var GlobalCategoryCache *CategoryCache
