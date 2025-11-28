package middleware

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"forum/server/cloud"
	"forum/server/models"
)

// UploadGatekeeper - Gatekeeper Pattern implementation
// Centralized entry point để kiểm soát upload permissions
type UploadGatekeeper struct {
	azureStorage *cloud.AzureStorage
	db           *sql.DB
	logger       *log.Logger
	// Rate limiter sẽ implement sau nếu cần
}

// UploadRequest - Request từ client để xin SAS token
type UploadRequest struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// UploadResponse - Response với SAS token
type UploadResponse struct {
	UploadURL string `json:"upload_url"` // Presigned URL để upload
	PublicURL string `json:"public_url"` // Public URL sau khi validate xong
	ObjectKey string `json:"object_key"` // Blob name trong Azure
	ExpiresIn int    `json:"expires_in"` // Seconds until token expires
}

// NewUploadGatekeeper tạo gatekeeper mới
func NewUploadGatekeeper(azure *cloud.AzureStorage, db *sql.DB) *UploadGatekeeper {
	return &UploadGatekeeper{
		azureStorage: azure,
		db:           db,
		logger:       log.New(os.Stdout, "[GATEKEEPER] ", log.LstdFlags),
	}
}

// GenerateUploadURL - Main handler, implements 4-step gatekeeper process
func (g *UploadGatekeeper) GenerateUploadURL(w http.ResponseWriter, r *http.Request) {
	g.logger.Println("Received upload request")

	// Step 1: Authentication check
	userID, username, valid := models.ValidSession(r, g.db)
	if !valid {
		g.logger.Println("✗ Unauthorized: No valid session")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	g.logger.Printf("✓ User authenticated: %s (ID: %d)", username, userID)

	// Step 2: Rate limiting (TODO: implement nếu cần)
	// For now, skip rate limiting

	// Step 3: Parse request
	var req UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.logger.Printf("✗ Invalid request body: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	g.logger.Printf("Request: file=%s, size=%d, type=%s", req.Filename, req.Size, req.ContentType)

	// Step 4: Metadata validation (backup check)
	if err := g.validateMetadata(req); err != nil {
		g.logger.Printf("✗ Metadata validation failed: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	g.logger.Println("✓ Metadata validation passed")

	// Step 5: Generate unique blob name
	ext := filepath.Ext(req.Filename)
	timestamp := time.Now().UnixNano()
	quarantineBlob := fmt.Sprintf("quarantine/%d_%d%s", userID, timestamp, ext)
	productionBlob := fmt.Sprintf("%d_%d%s", userID, timestamp, ext)

	g.logger.Printf("Generated blob names: quarantine=%s, production=%s", quarantineBlob, productionBlob)

	// Step 6: Generate SAS token for QUARANTINE container
	quarantineContainer := os.Getenv("AZURE_QUARANTINE_CONTAINER")
	if quarantineContainer == "" {
		quarantineContainer = "quarantine-container"
	}

	sasURL, err := g.azureStorage.GenerateSASToken(quarantineContainer, quarantineBlob, 5)
	if err != nil {
		g.logger.Printf("✗ Failed to generate SAS token: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Step 7: Build public URL (will be available after validation)
	productionContainer := os.Getenv("AZURE_PRODUCTION_CONTAINER")
	if productionContainer == "" {
		productionContainer = "post-images"
	}

	publicURL := g.azureStorage.GetPublicURL(productionContainer, productionBlob)

	g.logger.Printf("✓ SAS token generated (expires in 5min)")

	// Step 8: Return response
	response := UploadResponse{
		UploadURL: sasURL,
		PublicURL: publicURL,
		ObjectKey: quarantineBlob,
		ExpiresIn: 300, // 5 minutes
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	g.logger.Printf("✓ Response sent to client")
}

// validateMetadata - Lightweight validation on metadata only
// Đây chỉ là backup check, validation thật ở server content validation
func (g *UploadGatekeeper) validateMetadata(req UploadRequest) error {
	// Size check
	maxSize := int64(5 * 1024 * 1024) // 5MB
	if req.Size > maxSize {
		return fmt.Errorf("file size %d bytes exceeds maximum %d bytes", req.Size, maxSize)
	}

	// Extension check
	ext := strings.ToLower(filepath.Ext(req.Filename))
	allowedExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true,
		".gif": true, ".webp": true,
	}

	if !allowedExts[ext] {
		return fmt.Errorf("file extension %s not allowed", ext)
	}

	// MIME type check
	if !strings.HasPrefix(req.ContentType, "image/") {
		return fmt.Errorf("content type %s not allowed (must be image/*)", req.ContentType)
	}

	return nil
}
