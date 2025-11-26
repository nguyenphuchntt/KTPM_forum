package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // HTTP Metrics
    HttpRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "forum_http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )
    
    HttpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "forum_http_request_duration_seconds",
            Help: "HTTP request latency in seconds",
            Buckets: prometheus.DefBuckets, // [.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10]
        },
        []string{"method", "endpoint"},
    )
    
    // Cache Metrics
    CacheHitsTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "forum_cache_hits_total",
            Help: "Total number of cache hits",
        },
    )
    
    CacheMissesTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "forum_cache_misses_total",
            Help: "Total number of cache misses",
        },
    )
    
    CacheSize = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "forum_cache_size",
            Help: "Current number of items in cache",
        },
    )

	DbConnectionsInUse = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "forum_db_connections_in_use",
            Help: "Current number of DB connections in use",
        },
    )
    
    DbConnectionsIdle = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "forum_db_connections_idle",
            Help: "Current number of idle DB connections",
        },
    )
    
    DbConnectionsOpen = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "forum_db_connections_open",
            Help: "Current number of open DB connections",
        },
    )
    
    DbQueryDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "forum_db_query_duration_seconds",
            Help: "Database query duration in seconds",
            Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
        },
        []string{"query_type"},
    )
    
    DbQueryErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "forum_db_query_errors_total",
            Help: "Total number of database query errors",
        },
        []string{"query_type"},
    )
)

func NormalizeEndpoint(path string) string {
    // Replace dynamic IDs with placeholder
    // /post/123 -> /post/:id
    // /category/5 -> /category/:id
    
    // Simple implementation - you can make it more sophisticated
    if len(path) > 6 && path[:6] == "/post/" {
        return "/post/:id"
    }
    if len(path) > 10 && path[:10] == "/category/" {
        return "/category/:id"
    }
    return path
}