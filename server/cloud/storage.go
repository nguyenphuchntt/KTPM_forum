package cloud

import (
	"io"
)

// Storage interface cho tất cả storage implementations
// Cho phép dễ dàng switch giữa Local, Azure, AWS S3, etc.
type Storage interface {
	// GenerateSASToken tạo Shared Access Signature để client upload trực tiếp
	// containerName: tên container (quarantine hoặc production)
	// blobName: tên file trong container
	// duration: thời gian hết hạn của token
	// Returns: presigned URL với SAS token
	GenerateSASToken(containerName, blobName string, duration int) (string, error)

	// Download file từ Azure Storage
	// containerName: tên container
	// blobName: tên file
	// Returns: io.ReadCloser của file content
	Download(containerName, blobName string) (io.ReadCloser, error)

	// Copy file từ container này sang container khác (server-side copy, fast!)
	// srcContainer: container nguồn
	// srcBlob: file nguồn
	// dstContainer: container đích
	// dstBlob: file đích
	Copy(srcContainer, srcBlob, dstContainer, dstBlob string) error

	// Delete file khỏi container
	// containerName: tên container
	// blobName: tên file
	Delete(containerName, blobName string) error

	// GetPublicURL lấy URL public để serve image
	// containerName: tên container (thường là production)
	// blobName: tên file
	// Returns: full public URL
	GetPublicURL(containerName, blobName string) string

	// GetAccountName trả về tên storage account (for debugging)
	GetAccountName() string
}
