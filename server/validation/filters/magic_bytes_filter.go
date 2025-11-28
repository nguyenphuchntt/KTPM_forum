package filters

import (
	"bytes"
	"fmt"
	"io"

	"forum/server/validation"
)

// MagicBytesFilter kiểm tra file signature (magic bytes)
// Đây là filter BẢO MẬT quan trọng nhất - detect file giả mạo
type MagicBytesFilter struct {
	Signatures map[string][]byte // map[fileType]signature
}

// NewMagicBytesFilter tạo filter với các signature mặc định
func NewMagicBytesFilter() *MagicBytesFilter {
	return &MagicBytesFilter{
		Signatures: map[string][]byte{
			"jpeg": {0xFF, 0xD8, 0xFF},
			"png":  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			"gif":  {0x47, 0x49, 0x46, 0x38},
			"webp": {0x52, 0x49, 0x46, 0x46}, // Starts with RIFF
		},
	}
}

// Execute kiểm tra magic bytes của file
func (f *MagicBytesFilter) Execute(ctx *validation.ValidationContext) error {
	if ctx.Reader == nil {
		return fmt.Errorf("file content required for magic bytes validation")
	}

	// Đọc 512 bytes đầu tiên
	buffer := make([]byte, 512)
	n, err := ctx.Reader.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	buffer = buffer[:n]

	// Kiểm tra với từng signature đã biết
	for fileType, signature := range f.Signatures {
		if bytes.HasPrefix(buffer, signature) {
			// Match! File signature hợp lệ
			ctx.Metadata["detected_type"] = fileType
			ctx.Metadata["magic_bytes_checked"] = true
			return nil
		}
	}

	// Không match với bất kỳ signature nào - file giả mạo!
	hexDump := bytesToHex(buffer[:min(8, len(buffer))])
	return fmt.Errorf("invalid file signature: %s (not a real image)", hexDump)
}

// Helper: convert bytes to hex string
func bytesToHex(bytes []byte) string {
	result := ""
	for i, b := range bytes {
		if i > 0 {
			result += " "
		}
		result += fmt.Sprintf("%02X", b)
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
