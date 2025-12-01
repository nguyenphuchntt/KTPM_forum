package cloud

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// ConfigureAzureStorage sets up CORS and Public Access policies for the storage account
func ConfigureAzureStorage(connectionString, publicContainerName string) {
	// Parse connection string
	parts := parseConnectionString(connectionString)
	accountName := parts["AccountName"]
	accountKey := parts["AccountKey"]

	if accountName == "" || accountKey == "" {
		log.Println("Warning: Invalid Azure connection string, skipping configuration")
		return
	}

	// Create credential
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Printf("Warning: Invalid Azure credentials: %v", err)
		return
	}

	// Create pipeline
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))
	serviceURL := azblob.NewServiceURL(*u, p)

	// 1. Configure CORS
	configureCORS(serviceURL, accountName)

	// 2. Configure Public Access
	configurePublicAccess(serviceURL, publicContainerName)
}

func configureCORS(serviceURL azblob.ServiceURL, accountName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	corsRules := []azblob.CorsRule{
		{
			AllowedOrigins:  "*",
			AllowedMethods:  "GET,HEAD,POST,PUT,OPTIONS,DELETE",
			AllowedHeaders:  "*",
			ExposedHeaders:  "*",
			MaxAgeInSeconds: 3600,
		},
	}

	_, err := serviceURL.SetProperties(ctx, azblob.StorageServiceProperties{
		Cors: corsRules,
	})

	if err != nil {
		log.Printf("Warning: Failed to set CORS rules: %v", err)
	} else {
		log.Printf("✓ Azure CORS configured for account: %s", accountName)
	}
}

func configurePublicAccess(serviceURL azblob.ServiceURL, containerName string) {
	if containerName == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	containerURL := serviceURL.NewContainerURL(containerName)

	// Check if container exists first
	_, err := containerURL.GetProperties(ctx, azblob.LeaseAccessConditions{})
	if err != nil {
		// Try to create it if it doesn't exist
		_, err = containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessBlob)
		if err != nil {
			// If error is not "ContainerAlreadyExists", log warning
			if !strings.Contains(err.Error(), "ContainerAlreadyExists") {
				log.Printf("Warning: Failed to ensure container %s exists: %v", containerName, err)
				return
			}
		}
	}

	// Set Access Policy to Blob (Public Read for Blobs only)
	_, err = containerURL.SetAccessPolicy(ctx, azblob.PublicAccessBlob, []azblob.SignedIdentifier{}, azblob.ContainerAccessConditions{})
	if err != nil {
		log.Printf("Warning: Failed to set public access policy for %s: %v", containerName, err)
	} else {
		log.Printf("✓ Public access configured for container: %s", containerName)
	}
}


