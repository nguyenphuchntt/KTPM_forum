package workers

import (
	"context"
	"fmt"
	"forum/server/validation"
	"forum/server/validation/filters"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// StartQuarantineWatcher starts a background worker that monitors the quarantine container
// and automatically promotes valid files to the production container.
func StartQuarantineWatcher(connectionString string) {
	go func() {
		log.Println("[WATCHER] Starting Quarantine Watcher...")

		quarantineContainer := os.Getenv("AZURE_QUARANTINE_CONTAINER")
		if quarantineContainer == "" {
			quarantineContainer = "quarantine-container"
		}

		productionContainer := os.Getenv("AZURE_PRODUCTION_CONTAINER")
		if productionContainer == "" {
			productionContainer = "post-images"
		}

		// Parse connection string
		parts := parseConnectionString(connectionString)
		accountName := parts["AccountName"]
		accountKey := parts["AccountKey"]

		// Create credential
		credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
		if err != nil {
			log.Printf("[WATCHER] Error: Invalid credentials: %v", err)
			return
		}

		// Create pipeline
		p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

		// Create URL
		u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))
		serviceURL := azblob.NewServiceURL(*u, p)
		quarantineURL := serviceURL.NewContainerURL(quarantineContainer)
		productionURL := serviceURL.NewContainerURL(productionContainer)

		log.Printf("[WATCHER] Watching container '%s' for new files...", quarantineContainer)

		// Polling loop
		for {
			// List blobs
			ctx := context.Background()
			marker := azblob.Marker{}
			
			for {
				listBlob, err := quarantineURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
				if err != nil {
					log.Printf("[WATCHER] Error listing blobs: %v", err)
					time.Sleep(2 * time.Second)
					break
				}

				for _, blob := range listBlob.Segment.BlobItems {
					processBlob(ctx, serviceURL, quarantineURL, productionURL, blob.Name)
				}

				marker = listBlob.NextMarker
				if marker.Val == nil || *marker.Val == "" {
					break
				}
			}

			// Wait before next poll
			pollIntervalStr := os.Getenv("QUARANTINE_POLL_INTERVAL")
			pollInterval := 1 * time.Second // Default to 1s for better UX
			
			if val, err := time.ParseDuration(pollIntervalStr); err == nil {
				pollInterval = val
			}
			
			time.Sleep(pollInterval)
		}
	}()
}

func processBlob(ctx context.Context, serviceURL azblob.ServiceURL, quarantineURL, productionURL azblob.ContainerURL, blobName string) {
	log.Printf("[WATCHER] Processing %s...", blobName)

	// 1. Download
	blobURL := quarantineURL.NewBlobURL(blobName)
	downloadResponse, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		log.Printf("[WATCHER] Failed to download %s: %v", blobName, err)
		return
	}
	reader := downloadResponse.Body(azblob.RetryReaderOptions{MaxRetryRequests: 3})
	defer reader.Close()

	// Save to temp file
	tempFile, err := os.CreateTemp("", "promote-*")
	if err != nil {
		log.Printf("[WATCHER] Failed to create temp file: %v", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, reader); err != nil {
		log.Printf("[WATCHER] Failed to write temp file: %v", err)
		return
	}

	// Reset pointer
	if _, err := tempFile.Seek(0, 0); err != nil {
		log.Printf("[WATCHER] Failed to seek temp file: %v", err)
		return
	}

	// 2. Validate
	pipeline := validation.NewPipeline()
	pipeline.AddFilter(filters.NewMagicBytesFilter())
	pipeline.AddFilter(&filters.ImageIntegrityFilter{})

	valCtx := &validation.ValidationContext{
		Filename: blobName,
		Reader:   tempFile,
		Metadata: make(map[string]interface{}),
	}

	if err := pipeline.Execute(valCtx); err != nil {
		log.Printf("[WATCHER] Validation FAILED for %s: %v. Deleting...", blobName, err)
		blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
		return
	}

	log.Printf("[WATCHER] Validation PASSED for %s. Promoting...", blobName)

	// 3. Promote (Copy)
	// Strip "quarantine/" prefix for the destination
	destBlobName := strings.TrimPrefix(blobName, "quarantine/")
	destBlobURL := productionURL.NewBlobURL(destBlobName)
	
	// StartCopyFromURL is async, but usually fast for small files
	_, err = destBlobURL.StartCopyFromURL(ctx, blobURL.URL(), azblob.Metadata{}, azblob.ModifiedAccessConditions{}, azblob.BlobAccessConditions{}, azblob.DefaultAccessTier, nil)
	if err != nil {
		log.Printf("[WATCHER] Failed to copy %s: %v", blobName, err)
		return
	}

	// 4. Delete from Quarantine
	_, err = blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	if err != nil {
		log.Printf("[WATCHER] Warning: Failed to delete %s from quarantine: %v", blobName, err)
	} else {
		log.Printf("[WATCHER] Successfully promoted %s", blobName)
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
