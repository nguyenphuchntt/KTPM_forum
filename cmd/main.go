package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"forum/server/cloud"
	"forum/server/config"
	"forum/server/logger"
	"forum/server/middleware"
	"forum/server/routes"
	"forum/server/utils"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
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

	// Handle command-line flags for database setup if passed
	if len(os.Args) > 1 {
		if err := utils.HandleFlags(os.Args[1:], db); err != nil {
			fmt.Println(err)
			utils.Usage()
			os.Exit(1)
		}
		return
	}

	if isDocker {
		// Create the database schema and demo data for docker environment
		err := config.CreateDemoData(db)
		if err != nil {
			logger.Log.Fatal().Err(err).Msg("Error creating the database schema and demo data")
		}
		logger.Log.Info().Msg("Database setup complete")
	}



	// Initialize MinIO object storage for image uploads. Without an endpoint the
	// app still runs, with the upload routes simply not registered.
	storageConfig := config.LoadMinIOConfigFromEnv()
	var objectStorage cloud.Storage

	if storageConfig.Enabled() {
		minioStorage, err := cloud.NewMinIOStorage(storageConfig)
		if err != nil {
			logger.Log.Fatal().Err(err).Msg("Invalid MinIO configuration")
		}

		// Fail fast, but not immediately: AIStor validates its license before it
		// starts listening, and docker compose only guarantees the container
		// started. A configured-but-unreachable storage would otherwise only show
		// up later as a broken upload.
		if err := minioStorage.EnsureBucketsWithRetry(context.Background(), 30, time.Second); err != nil {
			logger.Log.Fatal().Err(err).Msg("MinIO is not usable")
		}

		objectStorage = minioStorage
		logger.Log.Info().
			Str("endpoint", storageConfig.Endpoint).
			Str("public_endpoint", storageConfig.PublicEndpoint).
			Str("quarantine_bucket", storageConfig.QuarantineBucket).
			Str("media_bucket", storageConfig.MediaBucket).
			Msg("Object storage ready")
	} else {
		logger.Log.Warn().Msg("MINIO_ENDPOINT is not set, image upload is disabled")
	}

	// Initialize rate limit config
	rateLimitConfig := config.DefaultRateLimitConfig()

	// Initialize global rate limiting middleware
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(db, rateLimitConfig)

	// Start the HTTP server with rate limiting
	handler := rateLimitMiddleware.Limit(routes.Routes(db, storageConfig, objectStorage))

	server := http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	logger.Log.Info().Msg("Server starting on http://localhost:8080")
	logger.Log.Info().Msg("Rate limiting enabled: Global + Per-User/IP + Endpoint-specific")
	if err := server.ListenAndServe(); err != nil {
		logger.Log.Fatal().Err(err).Msg("Server error")
	}
}

