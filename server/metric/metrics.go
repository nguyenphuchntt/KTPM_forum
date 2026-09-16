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
			Name:    "forum_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
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
			Name:    "forum_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
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

	DbConnectionWaitCount = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_db_connection_wait_count",
			Help: "Total number of connections waited for",
		},
	)

	DbConnectionWaitDuration = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_db_connection_wait_duration_seconds",
			Help: "Total time blocked waiting for new connections",
		},
	)

	DbConnectionMaxOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_db_connection_max_open",
			Help: "Maximum number of open connections to the database",
		},
	)

	DbConnectionMaxIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_db_connection_max_idle",
			Help: "Maximum number of idle connections in the pool",
		},
	)

	// Runtime Metrics
	GoGoroutines = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_go_goroutines",
			Help: "Number of goroutines that currently exist",
		},
	)

	GoMemoryHeapAlloc = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_go_memory_heap_alloc_bytes",
			Help: "Bytes allocated and still in use (heap)",
		},
	)

	GoMemoryHeapInuse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_go_memory_heap_inuse_bytes",
			Help: "Bytes in in-use spans",
		},
	)

	GoMemoryHeapSys = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_go_memory_heap_sys_bytes",
			Help: "Bytes obtained from system for heap",
		},
	)

	GoMemoryStackInuse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_go_memory_stack_inuse_bytes",
			Help: "Bytes in stack spans",
		},
	)

	GoGCPauseSeconds = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "forum_go_gc_pause_seconds",
			Help:    "GC pause duration in seconds",
			Buckets: []float64{0.00001, 0.00005, 0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1},
		},
	)

	GoGCCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "forum_go_gc_count_total",
			Help: "Total number of GC cycles",
		},
	)

	// Uptime and Availability Metrics
	ProcessStartTimeSeconds = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_process_start_time_seconds",
			Help: "Start time of the process since unix epoch in seconds",
		},
	)

	UptimeSeconds = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "forum_uptime_seconds_total",
			Help: "Total uptime of the application in seconds",
		},
	)

	// In-flight Requests
	HttpInFlightRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "forum_http_in_flight_requests",
			Help: "Current number of HTTP requests being processed",
		},
	)

	// Rate Limit Metrics
	RateLimitDropsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "forum_rate_limit_drops_total",
			Help: "Total number of requests dropped due to rate limiting",
		},
		[]string{"endpoint", "limiter_type"},
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
