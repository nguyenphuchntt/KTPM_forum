package controllers

import (
	"net/http"
	"strconv"

	"forum/server/dto/response"
	"forum/server/model"
	"forum/server/usecase"
)

// MediaController serves stored images. Templates render <img src="/api/v1/medias/{id}">
// (see model.Post.ImagePath), so this route is what puts an uploaded picture on
// the page.
type MediaController struct {
	Media *usecase.UploadUsecase
}

func NewMediaController(media *usecase.UploadUsecase) MediaController {
	return MediaController{Media: media}
}

func RegisterMediaRoutes(mux *http.ServeMux, c MediaController) {
	mux.HandleFunc("/api/v1/medias/{id}", c.ShowMedia)
}

// ShowMedia handles GET /api/v1/medias/{id} by redirecting to the object in
// storage. A permanent cache header is safe because a media row is immutable:
// its id always resolves to the same object.
func (c MediaController) ShowMedia(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	mediaID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || mediaID <= 0 {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid media id", nil))
		return
	}

	publicURL, err := c.Media.ResolveMediaURL(model.MediaID(mediaID))
	if err != nil {
		writeAppError(w, err)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.Redirect(w, r, publicURL, http.StatusFound)
}
