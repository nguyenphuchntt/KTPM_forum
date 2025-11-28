package validation

import (
	"io"
)

// ValidationContext chứa thông tin về file đang được validate
type ValidationContext struct {
	Reader   io.ReadSeeker          // File content stream (ReadSeeker required for filters)
	Filename string                 // Tên file
	Size     int64                  // Kích thước file
	MIMEType string                 // MIME type
	UserID   int                    // ID của user upload
	Metadata map[string]interface{} // Metadata bổ sung
}
