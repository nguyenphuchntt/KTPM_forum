package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter interface for different rate limiting strategies
type RateLimiter interface {
	Allow(key string) bool
	Reset(key string)
}

// MemoryRateLimiter implements rate limiting using in-memory storage
type MemoryRateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewMemoryRateLimiter creates a new in-memory rate limiter
// requestsPerSecond: number of requests allowed per second
// burst: maximum burst size (tokens bucket can hold)
func NewMemoryRateLimiter(requestsPerSecond int, burst int) *MemoryRateLimiter {
	limiter := &MemoryRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
	}

	// Cleanup expired limiters every 5 minutes
	go limiter.cleanup()

	return limiter
}

// getLimiter retrieves or creates a rate limiter for a given key
func (m *MemoryRateLimiter) getLimiter(key string) *rate.Limiter {
	m.mu.Lock()
	defer m.mu.Unlock()

	limiter, exists := m.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(m.rate, m.burst)
		m.limiters[key] = limiter
	}

	return limiter
}

// Allow checks if a request is allowed for the given key
func (m *MemoryRateLimiter) Allow(key string) bool {
	limiter := m.getLimiter(key)
	return limiter.Allow() // nếu còn token -> true, else false
}

// Reset removes the rate limiter for a given key
func (m *MemoryRateLimiter) Reset(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.limiters, key)
}

// cleanup periodically removes old limiters to prevent memory leaks
func (m *MemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		// If too many limiters, clear all (simple strategy)
		if len(m.limiters) > 10000 {
			m.limiters = make(map[string]*rate.Limiter)
		}
		m.mu.Unlock()
	}
}

// WindowRateLimiter implements sliding window rate limiting
type WindowRateLimiter struct {
	requests      map[string][]time.Time
	mu            sync.RWMutex
	maxRequests   int
	windowSize    time.Duration
}

// NewWindowRateLimiter creates a new sliding window rate limiter
func NewWindowRateLimiter(maxRequests int, windowSize time.Duration) *WindowRateLimiter {
	limiter := &WindowRateLimiter{
		requests:    make(map[string][]time.Time),
		maxRequests: maxRequests,
		windowSize:  windowSize,
	}

	// Cleanup old entries every minute
	go limiter.cleanup()

	return limiter
}

// Allow checks if a request is allowed within the sliding window
func (w *WindowRateLimiter) Allow(key string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-w.windowSize)

	// Get or create request history for this key
	timestamps, exists := w.requests[key]
	if !exists {
		timestamps = []time.Time{}
	}

	// Remove timestamps outside the window
	validTimestamps := []time.Time{}
	for _, ts := range timestamps {
		if ts.After(windowStart) {
			validTimestamps = append(validTimestamps, ts)
		}
	}

	// Check if we're within the limit
	if len(validTimestamps) >= w.maxRequests {
		w.requests[key] = validTimestamps
		return false
	}

	// Add current timestamp
	validTimestamps = append(validTimestamps, now)
	w.requests[key] = validTimestamps

	return true
}

// Reset removes the rate limit history for a given key
func (w *WindowRateLimiter) Reset(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.requests, key)
}

// cleanup periodically removes old entries
func (w *WindowRateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		w.mu.Lock()
		now := time.Now()
		windowStart := now.Add(-w.windowSize)

		for key, timestamps := range w.requests {
			validTimestamps := []time.Time{}
			for _, ts := range timestamps {
				if ts.After(windowStart) {
					validTimestamps = append(validTimestamps, ts)
				}
			}

			if len(validTimestamps) == 0 {
				delete(w.requests, key)
			} else {
				w.requests[key] = validTimestamps
			}
		}
		w.mu.Unlock()
	}
}
