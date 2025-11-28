package filters

import (
	"bytes"
	"errors"
	"forum/server/validation"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
)

// ImageIntegrityFilter checks if the file is a valid image by decoding it
type ImageIntegrityFilter struct{}

func (f *ImageIntegrityFilter) Execute(ctx *validation.ValidationContext) error {
	// Reset reader to the beginning
	if _, err := ctx.Reader.Seek(0, io.SeekStart); err != nil {
		return errors.New("failed to seek file stream")
	}

	// Read the entire content into memory for decoding
	// Note: For very large files, this might be memory intensive, 
	// but we already limit file size in Gatekeeper/Upload.
	data, err := io.ReadAll(ctx.Reader)
	if err != nil {
		return errors.New("failed to read file content")
	}

	// Decode config only (faster than decoding whole image) to verify header integrity
	_, _, err = image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return errors.New("image integrity check failed: invalid image format or corrupted file")
	}

	// Reset reader again for next filters
	if _, err := ctx.Reader.Seek(0, io.SeekStart); err != nil {
		return errors.New("failed to reset file stream")
	}

	return nil
}
