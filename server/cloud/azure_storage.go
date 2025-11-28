package cloud

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// AzureStorage implements Storage interface for Azure Blob Storage
type AzureStorage struct {
	accountName string
	accountKey  string
	credential  *azblob.SharedKeyCredential
	serviceURL  azblob.ServiceURL
}

// NewAzureStorage tạo Azure Storage client từ connection string
// connectionString: format "DefaultEndpointsProtocol=https;AccountName=...;AccountKey=...;EndpointSuffix=core.windows.net"
func NewAzureStorage(connectionString string) (*AzureStorage, error) {
	// Parse connection string
	parts := parseConnectionString(connectionString)

	accountName, ok := parts["AccountName"]
	if !ok {
		return nil, fmt.Errorf("AccountName not found in connection string")
	}

	accountKey, ok := parts["AccountKey"]
	if !ok {
		return nil, fmt.Errorf("AccountKey not found in connection string")
	}

	// Create credential
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	// Create pipeline
	pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// Create service URL
	URL, err := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))
	if err != nil {
		return nil, fmt.Errorf("failed to parse service URL: %w", err)
	}

	serviceURL := azblob.NewServiceURL(*URL, pipeline)

	return &AzureStorage{
		accountName: accountName,
		accountKey:  accountKey,
		credential:  credential,
		serviceURL:  serviceURL,
	}, nil
}

// GenerateSASToken tạo Shared Access Signature cho direct client upload
// Implements Valet Key Pattern
func (a *AzureStorage) GenerateSASToken(containerName, blobName string, durationMinutes int) (string, error) {
	containerURL := a.serviceURL.NewContainerURL(containerName)
	blobURL := containerURL.NewBlobURL(blobName)

	// Set SAS permissions: Write + Create only (security!)
	permissions := azblob.BlobSASPermissions{
		Write:  true,
		Create: true,
	}

	// Calculate expiry time
	expiryTime := time.Now().UTC().Add(time.Duration(durationMinutes) * time.Minute)

	// Generate SAS token
	sasQueryParams, err := azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS, // HTTPS only for security
		ExpiryTime:    expiryTime,
		ContainerName: containerName,
		BlobName:      blobName,
		Permissions:   permissions.String(),
	}.NewSASQueryParameters(a.credential)

	if err != nil {
		return "", fmt.Errorf("failed to generate SAS token: %w", err)
	}

	// Construct full URL with SAS token
	sasURL := fmt.Sprintf("%s?%s", blobURL.String(), sasQueryParams.Encode())
	return sasURL, nil
}

// Download file từ Azure Storage
func (a *AzureStorage) Download(containerName, blobName string) (io.ReadCloser, error) {
	ctx := context.Background()

	containerURL := a.serviceURL.NewContainerURL(containerName)
	blobURL := containerURL.NewBlobURL(blobName)

	// Download blob
	downloadResponse, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download blob: %w", err)
	}

	return downloadResponse.Body(azblob.RetryReaderOptions{MaxRetryRequests: 3}), nil
}

// Copy file từ container này sang container khác (server-side copy)
// Efficient vì không cần download/upload, Azure copy internally
func (a *AzureStorage) Copy(srcContainer, srcBlob, dstContainer, dstBlob string) error {
	ctx := context.Background()

	// Source blob URL
	srcContainerURL := a.serviceURL.NewContainerURL(srcContainer)
	srcBlobURL := srcContainerURL.NewBlobURL(srcBlob)

	// Destination blob URL
	dstContainerURL := a.serviceURL.NewContainerURL(dstContainer)
	dstBlobURL := dstContainerURL.NewBlobURL(dstBlob)

	// Start copy (server-side)
	_, err := dstBlobURL.StartCopyFromURL(ctx, srcBlobURL.URL(), azblob.Metadata{}, azblob.ModifiedAccessConditions{}, azblob.BlobAccessConditions{}, azblob.DefaultAccessTier, nil)
	if err != nil {
		return fmt.Errorf("failed to copy blob: %w", err)
	}

	// Note: StartCopyFromURL is async, but for small files (~5MB) it completes instantly
	// For large files, would need to poll CopyStatus
	return nil
}

// Delete file khỏi Azure Storage
func (a *AzureStorage) Delete(containerName, blobName string) error {
	ctx := context.Background()

	containerURL := a.serviceURL.NewContainerURL(containerName)
	blobURL := containerURL.NewBlobURL(blobName)

	// Delete blob (include snapshots nếu có)
	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	if err != nil {
		return fmt.Errorf("failed to delete blob: %w", err)
	}

	return nil
}

// GetPublicURL trả về URL public để serve image
func (a *AzureStorage) GetPublicURL(containerName, blobName string) string {
	return fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s",
		a.accountName, containerName, blobName)
}

// GetAccountName trả về storage account name
func (a *AzureStorage) GetAccountName() string {
	return a.accountName
}

// parseConnectionString helper function để parse Azure connection string
// Format: "DefaultEndpointsProtocol=https;AccountName=xxx;AccountKey=yyy;EndpointSuffix=core.windows.net"
func parseConnectionString(connStr string) map[string]string {
	parts := make(map[string]string)

	// Split by semicolon
	pairs := strings.Split(connStr, ";")
	for _, pair := range pairs {
		// Split by equals sign
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}

	return parts
}
