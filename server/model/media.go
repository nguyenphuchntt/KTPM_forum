package model

import (
	"strings"
	"time"
)

// MaxMediaSize is the largest image the upload pipeline accepts, enforced both
// when the upload URL is handed out and again against the object that actually
// lands in the quarantine bucket. The client-side pipeline mirrors this number.
const MaxMediaSize int64 = 5 << 20 // 5 MiB

// allowedImageExtensions lists what may be uploaded. The extension only decides
// the object key; the real gate is the file signature check on confirm.
var allowedImageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
}

// ExtensionAllowed reports whether the extension may be uploaded. The comparison
// is case-insensitive.
func ExtensionAllowed(ext string) bool {
	return allowedImageExtensions[normalizeExtension(ext)]
}

// ImageMIMEType maps a detected image type ("jpeg", "png", …) to its MIME type.
// It returns "" for anything unknown.
func ImageMIMEType(detectedType string) string {
	switch detectedType {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	default:
		return ""
	}
}

// ExtensionMatchesType reports whether a filename extension agrees with the type
// sniffed from the file's magic bytes, so a PNG uploaded as .jpg is rejected.
func ExtensionMatchesType(ext, detectedType string) bool {
	switch normalizeExtension(ext) {
	case ".jpg", ".jpeg":
		return detectedType == "jpeg"
	case ".png":
		return detectedType == "png"
	case ".gif":
		return detectedType == "gif"
	case ".webp":
		return detectedType == "webp"
	default:
		return false
	}
}

func normalizeExtension(ext string) string {
	return strings.ToLower(ext)
}

type MediaID int64

type Media struct {
	ID        MediaID
	ObjectKey string
	PublicURL string
	MIMEType  string
	Size      int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
