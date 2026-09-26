package validators

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"forum/server/dto/request"
	"forum/server/model"
)

// BindRequestUploadRequest decodes the body of POST /api/upload/request-url.
func BindRequestUploadRequest(r *http.Request) (request.RequestUploadRequest, error) {
	var req request.RequestUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

// ValidateRequestUploadRequest checks the metadata the client claims. It is a
// cheap pre-flight, not a security boundary: MinIO accepts whatever the client
// PUTs, so the authoritative checks run on confirm against the real object.
func ValidateRequestUploadRequest(req request.RequestUploadRequest) map[string]string {
	details := make(map[string]string)

	if strings.TrimSpace(req.Filename) == "" {
		details["filename"] = "filename is required"
	} else if !model.ExtensionAllowed(filepath.Ext(req.Filename)) {
		details["filename"] = "file extension must be one of .jpg, .jpeg, .png, .gif, .webp"
	}

	if req.Size <= 0 {
		details["size"] = "size must be greater than 0"
	} else if req.Size > model.MaxMediaSize {
		details["size"] = "file exceeds the 5MB limit"
	}

	if !strings.HasPrefix(req.ContentType, "image/") {
		details["content_type"] = "content type must be image/*"
	}

	if len(details) > 0 {
		return details
	}
	return nil
}

// BindConfirmUploadRequest decodes the body of POST /api/upload/confirm.
func BindConfirmUploadRequest(r *http.Request) (request.ConfirmUploadRequest, error) {
	var req request.ConfirmUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

// ValidateConfirmUploadRequest only checks that a key was sent. Whether the key
// is well-formed and belongs to the caller is decided by the usecase.
func ValidateConfirmUploadRequest(req request.ConfirmUploadRequest) map[string]string {
	if strings.TrimSpace(req.ObjectKey) == "" {
		return map[string]string{"object_key": "object_key is required"}
	}
	return nil
}
