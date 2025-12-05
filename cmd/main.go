package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"database/sql"
	"time"
	"runtime"

	"forum/server/cache"
	"forum/server/cloud"
	"forum/server/config"
	"forum/server/controllers"
	"forum/server/middleware"
	"forum/server/routes"
	"forum/server/utils"
	"forum/server/logger"
	"forum/server/metrics"
	"forum/server/workers"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"

)

func main() {
	// Initialize logger first
	logger.Init()
	
	err := godotenv.Load()
	if err != nil {
		logger.Log.Warn().Msg("No .env file found, using environment variables")
	}
	
	// Check if running in Docker
	isDocker := os.Getenv("BASE_PATH") != ""
	if isDocker {
		config.BasePath = os.Getenv("BASE_PATH")
	}

	// Connect to the database
	db, err := config.Connect()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("Database connection error")
	}

	// Initialize session cache (5 second TTL to reduce DB load)
	cache.InitSessionCache(5 * time.Second)
	logger.Log.Info().Msg("Session cache initialized with 5s TTL")

	// Initialize category cache (5 minute TTL)
	cache.GlobalCategoryCache = cache.NewCategoryCache(5 * time.Minute)
	if err := cache.GlobalCategoryCache.LoadCategories(db); err != nil {
		logger.Log.Fatal().Err(err).Msg("Failed to initialize category cache")
	}
	cache.GlobalCategoryCache.StartAutoRefresh(db)
	logger.Log.Info().Msg("Category cache initialized with 5m TTL and auto-refresh")

	// Initialize Azure Storage (for image uploads)
	connectionString := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	var uploadGatekeeper *middleware.UploadGatekeeper
	var webhookController *controllers.WebhookController
	
	if connectionString != "" {
		azureStorage, err := cloud.NewAzureStorage(connectionString)
		if err != nil {
			log.Printf("Warning: Failed to initialize Azure Storage: %v", err)
			log.Println("Image upload will not be available")
		} else {
			log.Printf("✓ Azure Storage connected: %s", azureStorage.GetAccountName())
			
			// Auto-configure CORS and Public Access
			cloud.ConfigureAzureStorage(connectionString, os.Getenv("AZURE_PRODUCTION_CONTAINER"))
			
			uploadGatekeeper = middleware.NewUploadGatekeeper(azureStorage, db)
			webhookController = controllers.NewWebhookController(azureStorage)
		}
	} else {
		log.Println("Warning: AZURE_STORAGE_CONNECTION_STRING not set, image upload disabled")
	}

	// Start Quarantine Watcher (for local dev automation)
	if os.Getenv("ENABLE_QUARANTINE_WATCHER") == "true" && connectionString != "" {
		workers.StartQuarantineWatcher(connectionString)
	}
	// Handle database setup based on environment
	if isDocker {
		// Create the database schema and demo data
		err := config.CreateDemoData(db)
		if err != nil {
			logger.Log.Fatal().Err(err).Msg("Error creating the database schema and demo data")
		}
		logger.Log.Info().Msg("Database setup complete")
	} else {
		// Handle command-line flags for database setup
		if len(os.Args) > 1 {
			if err := utils.HandleFlags(os.Args[1:], db); err != nil {
				fmt.Println(err)
				utils.Usage()
				os.Exit(1)
			}
			return
		}
	}

	// Initialize rate limit config
	rateLimitConfig := config.DefaultRateLimitConfig()

	// Initialize global rate limiting middleware
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(db, rateLimitConfig)
	
	// Initialize uptime metrics
	metrics.ProcessStartTimeSeconds.Set(float64(time.Now().Unix()))
	
	// Start collecting DB connection stats
	go collectDBStats(db)
	
	// Start collecting runtime stats
	go collectRuntimeStats()
	
	// Start the HTTP server with rate limiting
	handler := middleware.MetricsMiddleware(rateLimitMiddleware.Limit(routes.Routes(db, uploadGatekeeper, webhookController)))
	
	server := http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	go func() {
        metricsServer := http.NewServeMux()
        metricsServer.Handle("/metrics", promhttp.Handler())
        
        logger.Log.Info().Msg("Metrics server starting on :9090")
        if err := http.ListenAndServe(":9090", metricsServer); err != nil {
            logger.Log.Fatal().Err(err).Msg("Metrics server failed")
        }
    }()

	logger.Log.Info().Msg("Server starting on http://localhost:8080")
	logger.Log.Info().Msg("Rate limiting enabled: Global + Per-User/IP + Endpoint-specific")
	if err := server.ListenAndServe(); err != nil {
		logger.Log.Fatal().Err(err).Msg("Server error")
	}
}

func collectDBStats(db *sql.DB) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        stats := db.Stats()
        metrics.DbConnectionsInUse.Set(float64(stats.InUse))
        metrics.DbConnectionsIdle.Set(float64(stats.Idle))
        metrics.DbConnectionsOpen.Set(float64(stats.OpenConnections))
    }
}

func collectRuntimeStats() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    var memStats runtime.MemStats
    var lastGCCount uint32
    var lastGCPauseNs uint64
    
    for range ticker.C {
        // Collect goroutine count
        metrics.GoGoroutines.Set(float64(runtime.NumGoroutine()))
        
        // Collect memory stats
        runtime.ReadMemStats(&memStats)
        metrics.GoMemoryHeapAlloc.Set(float64(memStats.Alloc))
        metrics.GoMemoryHeapInuse.Set(float64(memStats.HeapInuse))
        metrics.GoMemoryHeapSys.Set(float64(memStats.HeapSys))
        metrics.GoMemoryStackInuse.Set(float64(memStats.StackInuse))
        
        // Track GC metrics
        if memStats.NumGC > lastGCCount {
            // New GC cycle(s) occurred
            gcCountDiff := memStats.NumGC - lastGCCount
            for i := uint32(0); i < gcCountDiff; i++ {
                metrics.GoGCCount.Inc()
            }
            
            // Record most recent GC pause time
            if memStats.PauseNs[(memStats.NumGC+255)%256] > lastGCPauseNs {
                pauseSeconds := float64(memStats.PauseNs[(memStats.NumGC+255)%256]) / 1e9
                metrics.GoGCPauseSeconds.Observe(pauseSeconds)
                lastGCPauseNs = memStats.PauseNs[(memStats.NumGC+255)%256]
            }
            
            lastGCCount = memStats.NumGC
        }
        
        // Increment uptime counter
        metrics.UptimeSeconds.Add(10) // Add 10 seconds for each tick
    }
}