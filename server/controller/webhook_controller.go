package controllers

import (
	"encoding/json"
	"fmt"
	"forum/server/cloud"
	"forum/server/validation"
	"forum/server/validation/filters"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// EventGridEvent represents the structure of an Azure Event Grid event
type EventGridEvent struct {
	Id              string      `json:"id"`
	Topic           string      `json:"topic"`
	Subject         string      `json:"subject"`
	EventType       string      `json:"eventType"`
	EventTime       time.Time   `json:"eventTime"`
	Data            interface{} `json:"data"`
	DataVersion     string      `json:"dataVersion"`
	MetadataVersion string      `json:"metadataVersion"`
}

// StorageBlobCreatedEventData represents the data payload for a BlobCreated event
type StorageBlobCreatedEventData struct {
	Api                string `json:"api"`
	ClientRequestId    string `json:"clientRequestId"`
	RequestId          string `json:"requestId"`
	ETag               string `json:"eTag"`
	ContentType        string `json:"contentType"`
	ContentLength      int64  `json:"contentLength"`
	BlobType           string `json:"blobType"`
	Url                string `json:"url"`
	Sequencer          string `json:"sequencer"`
	StorageDiagnostics interface{} `json:"storageDiagnostics"`
}

type WebhookController struct {
	Storage cloud.Storage
}

func NewWebhookController(storage cloud.Storage) *WebhookController {
	return &WebhookController{
		Storage: storage,
	}
}

func (c *WebhookController) HandleBlobCreated(w http.ResponseWriter, r *http.Request) {
	// 1. Handle Validation Handshake (Subscription Validation)
	// Azure Event Grid sends a validation request when you first register the webhook.
	if r.Method == http.MethodOptions {
		w.Header().Set("WebHook-Allowed-Origin", "*") // Or specific origin
		w.WriteHeader(http.StatusOK)
		return
	}

	var events []EventGridEvent
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &events); err != nil {
		http.Error(w, "Failed to parse event grid events", http.StatusBadRequest)
		return
	}

	for _, event := range events {
		// Handle Subscription Validation Event (if sent as POST)
		if event.EventType == "Microsoft.EventGrid.SubscriptionValidationEvent" {
			var validationData map[string]string
			dataBytes, _ := json.Marshal(event.Data)
			json.Unmarshal(dataBytes, &validationData)
			
			response := map[string]string{
				"validationResponse": validationData["validationCode"],
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle Blob Created Event
		if event.EventType == "Microsoft.Storage.BlobCreated" {
			if err := c.processBlobCreated(event); err != nil {
				log.Printf("Error processing blob created event: %v", err)
				// Don't fail the request, just log error (idempotency)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (c *WebhookController) processBlobCreated(event EventGridEvent) error {
	// Extract data
	dataBytes, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}
	var blobData StorageBlobCreatedEventData
	if err := json.Unmarshal(dataBytes, &blobData); err != nil {
		return err
	}

	// Check if this event is from the Quarantine container
	quarantineContainer := os.Getenv("AZURE_QUARANTINE_CONTAINER")
	if !strings.Contains(blobData.Url, quarantineContainer) {
		log.Println("Ignoring event from non-quarantine container:", blobData.Url)
		return nil
	}

	// Extract blob name (object key) from URL
	// URL format: https://<account>.blob.core.windows.net/<container>/<blob>
	parts := strings.Split(blobData.Url, "/")
	blobName := parts[len(parts)-1]

	log.Printf("Processing new blob in quarantine: %s", blobName)

	// 1. Download from Quarantine
	reader, err := c.Storage.Download(quarantineContainer, blobName)
	if err != nil {
		return fmt.Errorf("failed to download blob: %v", err)
	}
	defer reader.Close()

	// Create a temporary file to hold the content for validation (seeking needed)
	tempFile, err := os.CreateTemp("", "quarantine-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, reader); err != nil {
		return fmt.Errorf("failed to write to temp file: %v", err)
	}

	// Reset temp file pointer to beginning
	if _, err := tempFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to reset temp file pointer: %v", err)
	}

	// 2. Validate
	pipeline := validation.NewPipeline()
	pipeline.AddFilter(filters.NewMagicBytesFilter())
	pipeline.AddFilter(&filters.ImageIntegrityFilter{})

	ctx := &validation.ValidationContext{
		Filename: blobName,
		Reader:   tempFile,
		// FileSize and ContentType can be populated if needed
	}

	if err := pipeline.Execute(ctx); err != nil {
		log.Printf("Validation FAILED for %s: %v. Deleting...", blobName, err)
		// Delete invalid file
		if delErr := c.Storage.Delete(quarantineContainer, blobName); delErr != nil {
			log.Printf("Failed to delete invalid blob: %v", delErr)
		}
		return fmt.Errorf("validation failed: %v", err)
	}

	log.Printf("Validation PASSED for %s. Promoting to public...", blobName)

	// 3. Promote (Copy to Public)
	publicContainer := os.Getenv("AZURE_PRODUCTION_CONTAINER")
	
	// Reset temp file for upload (or use CopyBlob API if available)
	// Since we have the content locally, we can upload it. 
	// Ideally, we use StartCopyFromURL for server-side copy, but we need SAS for source.
	// For simplicity, let's re-upload the validated content.
	if _, err := tempFile.Seek(0, 0); err != nil {
		return err
	}

	// We need to implement Upload in ImageStorage interface or use Azure SDK directly here?
	// Our ImageStorage interface currently has: GenerateSASToken, Download, Copy, Delete.
	// Let's use Copy() which should implement StartCopyFromURL.
	
	err = c.Storage.Copy(quarantineContainer, blobName, publicContainer, blobName)
	if err != nil {
		return fmt.Errorf("failed to copy blob to public: %v", err)
	}

	// 4. Delete from Quarantine
	if err := c.Storage.Delete(quarantineContainer, blobName); err != nil {
		log.Printf("Warning: Failed to delete blob from quarantine after promotion: %v", err)
	}

	log.Printf("Successfully promoted %s to public container", blobName)
	return nil
}
