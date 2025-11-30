package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	if connStr == "" {
		log.Fatal("AZURE_STORAGE_CONNECTION_STRING is not set")
	}

	containerName := os.Getenv("AZURE_QUARANTINE_CONTAINER")
	if containerName == "" {
		containerName = "quarantine-container"
	}

	// Parse connection string
	parts := parseConnectionString(connStr)
	accountName := parts["AccountName"]
	accountKey := parts["AccountKey"]

	// Create credential
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal("Invalid credentials:", err)
	}

	// Create pipeline
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// Create URL
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))
	serviceURL := azblob.NewServiceURL(*u, p)
	containerURL := serviceURL.NewContainerURL(containerName)

	log.Printf("Listing blobs in container: %s", containerName)

	// List blobs
	ctx := context.Background()
	marker := azblob.Marker{}
	for {
		listBlob, err := containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			log.Fatal("Failed to list blobs:", err)
		}

		if len(listBlob.Segment.BlobItems) == 0 {
			log.Println("No blobs found in quarantine.")
		}

		for _, blob := range listBlob.Segment.BlobItems {
			fmt.Printf("- %s (Size: %d bytes)\n", blob.Name, *blob.Properties.ContentLength)
		}

		if marker.Val == nil || *marker.Val == "" {
			break
		}
	}
}

func parseConnectionString(connStr string) map[string]string {
	parts := make(map[string]string)
	pairs := strings.Split(connStr, ";")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}
	return parts
}
