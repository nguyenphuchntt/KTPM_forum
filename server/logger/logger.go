package logger

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var Log zerolog.Logger

func Init() {
	// Pretty logging for development
	if os.Getenv("ENVIRONMENT") == "development" {
		Log = zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}).With().Timestamp().Caller().Logger()
	} else {
		// JSON logging for production
		Log = zerolog.New(os.Stdout).With().
			Timestamp().
			Str("service", "forum").
			Logger()
	}

	// Set global log level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if os.Getenv("LOG_LEVEL") == "debug" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}

// WithRequest creates a logger with request context
func WithRequest(r *http.Request, userID int) zerolog.Logger {
	return Log.With().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("ip", GetClientIP(r)).
		Int("user_id", userID).
		Logger()
}

// GetClientIP extracts the real client IP address
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies/load balancers)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}