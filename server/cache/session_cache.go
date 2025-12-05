package cache

import (
	"sync"
	"time"
)

type SessionCacheEntry struct {
	UserID     int
	Username   string
	ExpiresAt  time.Time
	CachedAt   time.Time
}

type SessionCache struct {
	mu    sync.RWMutex
	cache map[string]SessionCacheEntry
	ttl   time.Duration
}

var GlobalSessionCache *SessionCache

func InitSessionCache(ttl time.Duration) {
	GlobalSessionCache = &SessionCache{
		cache: make(map[string]SessionCacheEntry),
		ttl:   ttl,
	}
	go GlobalSessionCache.cleanup()
}

func (sc *SessionCache) Get(sessionID string) (SessionCacheEntry, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	entry, exists := sc.cache[sessionID]
	if !exists {
		return SessionCacheEntry{}, false
	}

	if time.Now().After(entry.CachedAt.Add(sc.ttl)) {
		return SessionCacheEntry{}, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return SessionCacheEntry{}, false
	}

	return entry, true
}

func (sc *SessionCache) Set(sessionID string, userID int, username string, expiresAt time.Time) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.cache[sessionID] = SessionCacheEntry{
		UserID:    userID,
		Username:  username,
		ExpiresAt: expiresAt,
		CachedAt:  time.Now(),
	}
}

func (sc *SessionCache) Delete(sessionID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	delete(sc.cache, sessionID)
}

func (sc *SessionCache) DeleteByUserID(userID int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	for sessionID, entry := range sc.cache {
		if entry.UserID == userID {
			delete(sc.cache, sessionID)
		}
	}
}

func (sc *SessionCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sc.mu.Lock()
		now := time.Now()
		for sessionID, entry := range sc.cache {
			if now.After(entry.CachedAt.Add(sc.ttl)) || now.After(entry.ExpiresAt) {
				delete(sc.cache, sessionID)
			}
		}
		sc.mu.Unlock()
	}
}
