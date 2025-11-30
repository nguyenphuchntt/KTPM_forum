package middleware

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"forum/server/config"
	"forum/server/metrics"
	"forum/server/middleware/ratelimit"
	"forum/server/models"
)

// RateLimitMiddleware manages rate limiting for the application
type RateLimitMiddleware struct {
	globalLimiter *ratelimit.MemoryRateLimiter
	userLimiter   *ratelimit.WindowRateLimiter
	ipLimiter     *ratelimit.WindowRateLimiter
	config        *config.RateLimitConfig
	db            *sql.DB
}

// NewRateLimitMiddleware creates a new rate limit middleware
func NewRateLimitMiddleware(db *sql.DB, cfg *config.RateLimitConfig) *RateLimitMiddleware {
	if cfg == nil {
		cfg = config.DefaultRateLimitConfig()
	}

	return &RateLimitMiddleware{
		// Global: configurable requests/second across all users
		globalLimiter: ratelimit.NewMemoryRateLimiter(cfg.GlobalRequestsPerSecond, cfg.GlobalBurst),

		// Per user: configurable requests per minute
		userLimiter: ratelimit.NewWindowRateLimiter(cfg.UserRequestsPerMinute, cfg.UserWindowSize),

		// Per IP: configurable requests per minute for anonymous
		ipLimiter: ratelimit.NewWindowRateLimiter(cfg.IPRequestsPerMinute, cfg.IPWindowSize),

		config: cfg,
		db:     db,
	}
}

// Limit applies rate limiting to all requests
func (m *RateLimitMiddleware) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting for static assets
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			next.ServeHTTP(w, r)
			return
		}

		// 1. Check global rate limit
		if !m.globalLimiter.Allow("global") {
			metrics.RateLimitDropsTotal.WithLabelValues(r.URL.Path, "global").Inc()
			m.sendRateLimitError(w, r, "Too many requests globally. Please try again later.")
			return
		}

		// 2. Get user identifier
		userID := m.getUserID(r)
		clientIP := m.getClientIP(r)

		// 3. Check per-user or per-IP limit
		var limitKey string
		var limiter ratelimit.RateLimiter

		if userID != "" {
			// Authenticated user
			limitKey = fmt.Sprintf("user:%s", userID)
			limiter = m.userLimiter
		} else {
			// Anonymous user - use IP
			limitKey = fmt.Sprintf("ip:%s", clientIP)
			limiter = m.ipLimiter
		}

		if !limiter.Allow(limitKey) {
			limiterType := "ip"
			if userID != "" {
				limiterType = "user"
			}
			metrics.RateLimitDropsTotal.WithLabelValues(r.URL.Path, limiterType).Inc()
			m.sendRateLimitError(w, r, "Rate limit exceeded. Please slow down.")
			return
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", "100")

		next.ServeHTTP(w, r)
	})
}

// getUserID extracts user ID from session
func (m *RateLimitMiddleware) getUserID(r *http.Request) string {
	userID, _, valid := models.ValidSession(r, m.db)
	if !valid {
		return ""
	}
	return fmt.Sprintf("%d", userID)
}

// getClientIP extracts the real client IP address
func (m *RateLimitMiddleware) getClientIP(r *http.Request) string {
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

// sendRateLimitError sends a rate limit error response
func (m *RateLimitMiddleware) sendRateLimitError(w http.ResponseWriter, r *http.Request, message string) {
	w.Header().Set("Retry-After", "60")
	
	// Log the rate limit violation
	log.Printf("RATE_LIMIT_EXCEEDED | IP: %s | Path: %s | Method: %s",
		m.getClientIP(r), r.URL.Path, r.Method)

	// Check if it's an AJAX/API request
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" || 
	   strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "rate_limit_exceeded",
			"message": message,
		})
		return
	}

	// For regular browser requests, return HTML error
	w.WriteHeader(http.StatusTooManyRequests)
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Rate Limit Exceeded</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
        h1 { color: #d32f2f; }
        p { color: #666; }
        a { color: #1976d2; text-decoration: none; }
    </style>
</head>
<body>
    <h1>429 - Too Many Requests</h1>
    <p>%s</p>
    <p>Please wait a moment before trying again.</p>
    <a href="/">Go to Homepage</a>
</body>
</html>`, message)
	w.Write([]byte(html))
}

// EndpointRateLimiter provides rate limiting for specific endpoints
type EndpointRateLimiter struct {
	loginLimiter    *ratelimit.WindowRateLimiter
	registerLimiter *ratelimit.WindowRateLimiter
	postLimiter     *ratelimit.WindowRateLimiter
	commentLimiter  *ratelimit.WindowRateLimiter
	uploadLimiter   *ratelimit.WindowRateLimiter
	config          *config.RateLimitConfig
	db              *sql.DB
}

// NewEndpointRateLimiter creates endpoint-specific rate limiters
func NewEndpointRateLimiter(db *sql.DB, cfg *config.RateLimitConfig) *EndpointRateLimiter {
	if cfg == nil {
		cfg = config.DefaultRateLimitConfig()
	}

	return &EndpointRateLimiter{
		// Login: configurable attempts per window
		loginLimiter: ratelimit.NewWindowRateLimiter(cfg.LoginAttemptsPerWindow, cfg.LoginWindowSize),

		// Register: configurable accounts per window
		registerLimiter: ratelimit.NewWindowRateLimiter(cfg.RegisterAttemptsPerWindow, cfg.RegisterWindowSize),

		// Create post: configurable posts per hour
		postLimiter: ratelimit.NewWindowRateLimiter(cfg.PostsPerHour, 1*time.Hour),

		// Create comment: configurable comments per hour
		commentLimiter: ratelimit.NewWindowRateLimiter(cfg.CommentsPerHour, 1*time.Hour),

		// Upload: configurable uploads per minute
		uploadLimiter: ratelimit.NewWindowRateLimiter(cfg.UploadRequestsPerMinute, 1*time.Minute),

		config: cfg,
		db:     db,
	}
}

// LimitLogin rate limits login attempts
func (e *EndpointRateLimiter) LimitLogin(next http.HandlerFunc, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		key := "login:" + ip

		if !e.loginLimiter.Allow(key) {
			log.Printf("[RATE_LIMIT] Type=login | IP=%s | Limit=%d/%v",
				ip, e.config.LoginAttemptsPerWindow, e.config.LoginWindowSize)
			
			metrics.RateLimitDropsTotal.WithLabelValues("/signin", "login").Inc()
			
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "rate_limit_exceeded",
				"message": "Quá nhiều lần đăng nhập. Vui lòng thử lại sau.",
			})
			return
		}

		next(w, r)
	}
}

// LimitRegister rate limits registration attempts
func (e *EndpointRateLimiter) LimitRegister(next http.HandlerFunc, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		key := "register:" + ip

		if !e.registerLimiter.Allow(key) {
			log.Printf("[RATE_LIMIT] Type=register | IP=%s | Limit=%d/%v",
				ip, e.config.RegisterAttemptsPerWindow, e.config.RegisterWindowSize)
			
			metrics.RateLimitDropsTotal.WithLabelValues("/signup", "register").Inc()
			
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "rate_limit_exceeded",
				"message": "Quá nhiều lần đăng ký. Vui lòng thử lại sau.",
			})
			return
		}

		next(w, r)
	}
}

// LimitCreatePost rate limits post creation
func (e *EndpointRateLimiter) LimitCreatePost(next http.HandlerFunc, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _, valid := models.ValidSession(r, db)
		if !valid {
			next(w, r)
			return
		}

		key := fmt.Sprintf("post:%d", userID)
		if !e.postLimiter.Allow(key) {
			log.Printf("[RATE_LIMIT] Type=post | UserID=%d | Limit=%d/hour",
				userID, e.config.PostsPerHour)
			
			metrics.RateLimitDropsTotal.WithLabelValues("/post/createpost", "post").Inc()
			
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "rate_limit_exceeded",
				"message": "Bạn đang tạo bài viết quá nhanh. Vui lòng thử lại sau.",
			})
			return
		}

		next(w, r)
	}
}

// LimitCreateComment rate limits comment creation
func (e *EndpointRateLimiter) LimitCreateComment(next http.HandlerFunc, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _, valid := models.ValidSession(r, db)
		if !valid {
			next(w, r)
			return
		}

		key := fmt.Sprintf("comment:%d", userID)
		if !e.commentLimiter.Allow(key) {
			log.Printf("[RATE_LIMIT] Type=comment | UserID=%d | Limit=%d/hour",
				userID, e.config.CommentsPerHour)
			
			metrics.RateLimitDropsTotal.WithLabelValues("/post/addcommentREQ", "comment").Inc()
			
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "rate_limit_exceeded",
				"message": "Bạn đang bình luận quá nhanh. Vui lòng thử lại sau.",
			})
			return
		}

		next(w, r)
	}
}

// LimitUpload rate limits file uploads
func (e *EndpointRateLimiter) LimitUpload(next http.Handler, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use IP for upload limiting to prevent spam from same source
		// (Even if authenticated, we want to limit by source IP for uploads as an extra layer)
		// Or better: use UserID if available, else IP.
		
		var key string
		userID, _, valid := models.ValidSession(r, db)
		if valid {
			key = fmt.Sprintf("upload:user:%d", userID)
		} else {
			ip := getClientIP(r)
			key = "upload:ip:" + ip
		}

		if !e.uploadLimiter.Allow(key) {
			log.Printf("UPLOAD_RATE_LIMIT | Key: %s", key)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Helper function to get client IP
func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}
