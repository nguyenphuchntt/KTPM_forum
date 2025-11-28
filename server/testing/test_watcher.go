package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env
	godotenv.Load()

	connStr := os.Getenv("AZURE_STORAGE_CONNECTION_STRING")
	if connStr == "" {
		log.Fatal("AZURE_STORAGE_CONNECTION_STRING is not set")
	}

	quarantineContainer := os.Getenv("AZURE_QUARANTINE_CONTAINER")
	if quarantineContainer == "" {
		quarantineContainer = "quarantine-container"
	}

	// Parse connection string
	parts := parseConnectionString(connStr)
	accountName := parts["AccountName"]
	accountKey := parts["AccountKey"]

	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal(err)
	}
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))
	serviceURL := azblob.NewServiceURL(*u, p)
	containerURL := serviceURL.NewContainerURL(quarantineContainer)

	// Upload a test file
	blobName := fmt.Sprintf("watcher_test_%d.jpg", time.Now().Unix())
	log.Printf("Uploading %s to quarantine...", blobName)
	
	// Read real image
	imagePath := "web/assets/images/bg.jpg"
	data, err := os.ReadFile(imagePath)
	if err != nil {
		log.Fatal(err)
	}

	blobURL := containerURL.NewBlockBlobURL(blobName)
	_, err = blobURL.Upload(context.Background(), bytes.NewReader(data), azblob.BlobHTTPHeaders{ContentType: "image/jpeg"}, azblob.Metadata{}, azblob.BlobAccessConditions{}, azblob.AccessTierNone, azblob.BlobTagsMap{}, azblob.ClientProvidedKeyOptions{}, azblob.ImmutabilityPolicyOptions{})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Upload complete. Waiting for watcher to promote (should happen in ~5s)...")

	// Poll for deletion
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		_, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
		if err != nil {
			if stgErr, ok := err.(azblob.StorageError); ok && stgErr.ServiceCode() == azblob.ServiceCodeBlobNotFound {
				log.Println("SUCCESS: Blob was removed from quarantine (Promoted!)")
				return
			}
		}
		fmt.Print(".")
	}

	log.Println("\nTIMEOUT: Blob was not removed from quarantine after 20s.")
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
