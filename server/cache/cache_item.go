package cache

import "time"

type CacheItem struct {
	Data      interface{}
	ExpiresAt int64
}

func IsExpired(item CacheItem) bool {
	return item.ExpiresAt > 0 && time.Now().UnixNano() > item.ExpiresAt
}

