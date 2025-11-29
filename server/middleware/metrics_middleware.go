package middleware

import (
    "net/http"
    "strconv"
    "time"
    
    "forum/server/metrics"
)

func MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Increment in-flight requests
        metrics.HttpInFlightRequests.Inc()
        defer metrics.HttpInFlightRequests.Dec()
        
        // Create response writer wrapper to capture status code
        wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        // Process request
        next.ServeHTTP(wrapped, r)
        
        // Record metrics
        duration := time.Since(start).Seconds()
        endpoint := metrics.NormalizeEndpoint(r.URL.Path)
        status := strconv.Itoa(wrapped.statusCode)
        
        metrics.HttpRequestsTotal.WithLabelValues(r.Method, endpoint, status).Inc()
        metrics.HttpRequestDuration.WithLabelValues(r.Method, endpoint).Observe(duration)
    })
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}