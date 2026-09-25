package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"forum/server/cloud"
	"forum/server/config"
	"forum/server/controller"
	"forum/server/logger"
	"forum/server/middleware"
	"forum/server/routes"
	"forum/server/utils"
	"forum/server/workers"

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

	// Initialize rate limit config
	rateLimitConfig := config.DefaultRateLimitConfig()

	// Initialize global rate limiting middleware
	rateLimitMiddleware := middleware.NewRateLimitMiddleware(db, rateLimitConfig)

	// Start the HTTP server with rate limiting
	handler := rateLimitMiddleware.Limit(routes.Routes(db, uploadGatekeeper, webhookController))

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

