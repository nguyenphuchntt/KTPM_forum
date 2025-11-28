package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	connectionString := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	quarantineContainer := os.Getenv("AZURE_QUARANTINE_CONTAINER")
	publicContainer := os.Getenv("AZURE_PRODUCTION_CONTAINER")

	if connectionString == "" || quarantineContainer == "" || publicContainer == "" {
		log.Fatal("Missing environment variables")
	}

	// 1. Setup Azure Client
	credential, err := azblob.NewSharedKeyCredential(
		getValueFromConnString(connectionString, "AccountName"),
		getValueFromConnString(connectionString, "AccountKey"),
	)
	if err != nil {
		log.Fatal("Invalid credentials:", err)
	}
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", getValueFromConnString(connectionString, "AccountName")))
	serviceURL := azblob.NewServiceURL(*u, p)

	// 2. Upload a test file to Quarantine
	blobName := fmt.Sprintf("test_webhook_%d.jpg", time.Now().Unix())
	containerURL := serviceURL.NewContainerURL(quarantineContainer)
	blobURL := containerURL.NewBlockBlobURL(blobName)

	// Create a dummy valid JPEG (minimal header)
	// FF D8 FF E0 00 10 4A 46 49 46 00 01 01 01 00 48 00 48 00 00 FF DB ...
	// This is just a magic byte check, ImageIntegrityFilter might need more real data.
	// Let's use a small valid jpeg hex sequence converted to bytes.
	// Or better, read a real file if available. For now, let's try a minimal valid JPEG.
	// Read a real image file
	imagePath := "web/assets/images/bg.jpg"
	validJpeg, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatalf("Failed to read test image %s: %v", imagePath, err)
	}

	fmt.Printf("Uploading %s to quarantine...\n", blobName)
	_, err = blobURL.Upload(context.Background(), bytes.NewReader(validJpeg), azblob.BlobHTTPHeaders{ContentType: "image/jpeg"}, azblob.Metadata{}, azblob.BlobAccessConditions{}, azblob.AccessTierNone, azblob.BlobTagsMap{}, azblob.ClientProvidedKeyOptions{}, azblob.ImmutabilityPolicyOptions{})
	if err != nil {
		log.Fatal("Upload failed:", err)
	}

	// 3. Construct Event Grid Payload
	eventPayload := []map[string]interface{}{
		{
			"id":              "test-event-id",
			"topic":           "/subscriptions/sub-id/resourceGroups/rg/providers/Microsoft.Storage/storageAccounts/" + getValueFromConnString(connectionString, "AccountName"),
			"subject":         "/blobServices/default/containers/" + quarantineContainer + "/blobs/" + blobName,
			"eventType":       "Microsoft.Storage.BlobCreated",
			"eventTime":       time.Now().Format(time.RFC3339),
			"dataVersion":     "1.0",
			"metadataVersion": "1",
			"data": map[string]interface{}{
				"api":             "PutBlockList",
				"clientRequestId": "test-req-id",
				"requestId":       "test-req-id",
				"eTag":            "0x8D8...",
				"contentType":     "image/jpeg",
				"contentLength":   len(validJpeg),
				"blobType":        "BlockBlob",
				"url":             blobURL.String(),
				"sequencer":       "0000000000000000000000000000999900000000001f9c9c",
			},
		},
	}

	payloadBytes, _ := json.Marshal(eventPayload)

	// 4. Send Webhook Request
	fmt.Println("Triggering webhook...")
	resp, err := http.Post("http://localhost:8080/api/webhook/blob-created", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		log.Fatal("Webhook request failed:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Webhook returned status: %d", resp.StatusCode)
	}
	fmt.Println("Webhook triggered successfully (200 OK)")

	// 5. Verify Promotion
	// Check if file exists in Public container
	fmt.Println("Verifying promotion...")
	time.Sleep(2 * time.Second) // Give it a moment

	publicBlobURL := serviceURL.NewContainerURL(publicContainer).NewBlockBlobURL(blobName)
	_, err = publicBlobURL.GetProperties(context.Background(), azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		log.Fatalf("Verification FAILED: Blob not found in public container: %v", err)
	}
	fmt.Println("✓ Blob found in public container")

	// Check if file is GONE from Quarantine container
	_, err = blobURL.GetProperties(context.Background(), azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	if err == nil {
		log.Fatalf("Verification FAILED: Blob still exists in quarantine container")
	}
	fmt.Println("✓ Blob removed from quarantine container")

	fmt.Println("SUCCESS: Webhook flow verified!")
}

func getValueFromConnString(connStr, key string) string {
	for _, part := range strings.Split(connStr, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && kv[0] == key {
			return kv[1]
		}
	}
	return ""
}
