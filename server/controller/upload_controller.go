package controllers

import (
	"net/http"

	"forum/server/dto/response"
	"forum/server/usecase"
	"forum/server/validators"
)

// UploadController exposes the valet-key upload flow: ask for a pre-signed PUT,
// then confirm the upload so the server can validate and publish it.
type UploadController struct {
	Upload *usecase.UploadUsecase
}

func NewUploadController(upload *usecase.UploadUsecase) UploadController {
	return UploadController{Upload: upload}
}

// RequestUploadURLJSON handles POST /api/upload/request-url.
func (c UploadController) RequestUploadURLJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	req, err := validators.BindRequestUploadRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "malformed request body", nil))
		return
	}

	if details := validators.ValidateRequestUploadRequest(req); details != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid upload request", details))
		return
	}

	ticket, err := c.Upload.RequestUploadURL(r, req)
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, ticket)
}

// ConfirmUploadJSON handles POST /api/upload/confirm. It answers with the media
// id to attach to a post and the URL the image is already reachable at.
func (c UploadController) ConfirmUploadJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	req, err := validators.BindConfirmUploadRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "malformed request body", nil))
		return
	}

	if details := validators.ValidateConfirmUploadRequest(req); details != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid upload confirmation", details))
		return
	}

	media, err := c.Upload.ConfirmUpload(r, req)
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, media)
}
