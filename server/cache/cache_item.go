package cache

import "time"

type CacheItem struct {
	Data      interface{}
	ExpiresAt time.Time
}

func IsExpired(item CacheItem) bool {
	if item.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(item.ExpiresAt)
}


