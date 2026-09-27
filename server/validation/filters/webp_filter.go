package filters

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"forum/server/validation"
)

// WebPFilter completes the magic-bytes check for WebP.
//
// MagicBytesFilter only matches the leading "RIFF", which a WAV or AVI file
// carries too, and the stdlib has no WebP decoder to fall back on the way
// ImageIntegrityFilter does for the other formats. This filter therefore reads
// the container header: RIFF, a 4-byte little-endian length, then "WEBP".
type WebPFilter struct{}

const (
	riffHeaderLen = 12
	// maxContainerProbe bounds how much of a claimed 4GiB container header we
	// are willing to buffer; anything above this is not a plausible image.
	maxContainerProbe = 1 << 20
)

func (f *WebPFilter) Execute(ctx *validation.ValidationContext) error {
	if ctx.Reader == nil {
		return errors.New("file content required for webp validation")
	}

	header := make([]byte, riffHeaderLen)
	if _, err := io.ReadFull(ctx.Reader, header); err != nil {
		return fmt.Errorf("webp header is truncated: %w", err)
	}

	if !bytes.Equal(header[0:4], []byte("RIFF")) || !bytes.Equal(header[8:12], []byte("WEBP")) {
		return errors.New("not a webp container")
	}

	// Bytes 4:8 are the RIFF chunk length: 4 ("WEBP") plus the payload.
	declared := int64(uint32(header[4]) | uint32(header[5])<<8 | uint32(header[6])<<16 | uint32(header[7])<<24)
	if declared <= 4 || declared > maxContainerProbe {
		return fmt.Errorf("implausible webp container length %d", declared)
	}

	// The declared payload must be present, otherwise the file was truncated.
	if ctx.Size > 0 && declared+8 > ctx.Size {
		return fmt.Errorf("webp container claims %d bytes but the file holds %d", declared+8, ctx.Size)
	}

	return nil
}
