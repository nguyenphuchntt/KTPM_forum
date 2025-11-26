package config

import (
	"os"
	"strconv"
	"time"
)

// RateLimitConfig holds configuration for rate limiting
type RateLimitConfig struct {
	// Global limits
	GlobalRequestsPerSecond int
	GlobalBurst             int

	// Per-user limits
	UserRequestsPerMinute int
	UserWindowSize        time.Duration

	// Per-IP limits (for anonymous users)
	IPRequestsPerMinute int
	IPWindowSize        time.Duration

	// Endpoint-specific limits
	LoginAttemptsPerWindow    int
	LoginWindowSize           time.Duration
	RegisterAttemptsPerWindow int
	RegisterWindowSize        time.Duration
	PostsPerHour              int
	CommentsPerHour           int
	ReactionsPerMinute        int

	// Feature flags
	EnableRateLimit         bool
	EnableEndpointLimits    bool
	LogRateLimitViolations  bool
}

// DefaultRateLimitConfig returns default rate limiting configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		// Global limits
		GlobalRequestsPerSecond: getEnvInt("GLOBAL_RPS", 100),
		GlobalBurst:             getEnvInt("GLOBAL_BURST", 150),

		// Per-user/IP limits
		UserRequestsPerMinute: getEnvInt("USER_RPM", 1000000000),
		UserWindowSize:        1 * time.Minute,
		IPRequestsPerMinute:   getEnvInt("IP_RPM", 1000000000),
		IPWindowSize:          1 * time.Minute,

	// Endpoint-specific limits
	LoginAttemptsPerWindow:    getEnvInt("LOGIN_ATTEMPTS", 5),
	LoginWindowSize:           time.Duration(getEnvInt("LOGIN_WINDOW_MINUTES", 15)) * time.Minute,
	RegisterAttemptsPerWindow: getEnvInt("REGISTER_ATTEMPTS", 3),
	RegisterWindowSize:        time.Duration(getEnvInt("REGISTER_WINDOW_MINUTES", 60)) * time.Minute,
	PostsPerHour:              getEnvInt("POSTS_PER_HOUR", 10),
	CommentsPerHour:           getEnvInt("COMMENTS_PER_HOUR", 30),
	ReactionsPerMinute:        getEnvInt("REACTIONS_PER_MINUTE", 60),
	}
}

// Helper functions for environment variables
func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal
		}
	}
	return defaultVal
}
