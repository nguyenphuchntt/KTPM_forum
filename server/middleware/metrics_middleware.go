package middleware

import (
	"net/http"
)

// MetricsMiddleware is temporarily disabled. Passes through directly to next handler.
func MetricsMiddleware(next http.Handler) http.Handler {
	return next
}
