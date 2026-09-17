package controllers

import (
	"net/http"
	"strconv"

	"forum/server/dto/response"
	"forum/server/logger"
	"forum/server/usecase"
	"forum/server/validators"
)

// CreateCommentJSON handles POST /api/v1/posts/{post_id}/comments.
// The post id comes from the URL; the author comes from the session.
func (c CommentController) CreateCommentJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	req, err := validators.BindCreateCommentRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "malformed request body", nil))
		return
	}

	// The path id is authoritative; a body-provided post_id is ignored.
	postID, err := strconv.ParseInt(r.PathValue("post_id"), 10, 64)
	if err != nil || postID <= 0 {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid post id", nil))
		return
	}
	req.PostID = postID

	if details := validators.ValidateCreateCommentRequest(req); details != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid comment payload", details))
		return
	}

	result, err := c.Comment.CreateComment(r, req)
	if err != nil {
		writeAppError(w, err)
		return
	}

	log := logger.WithRequest(r, result.Comment.UserID)
	log.Info().Msg("Comment created")
	writeJSON(w, http.StatusCreated, result)
}

// ListCommentsJSON handles GET /api/v1/posts/{post_id}/comments.
func (c CommentController) ListCommentsJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	postID, err := strconv.ParseInt(r.PathValue("post_id"), 10, 64)
	if err != nil || postID <= 0 {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid post id", nil))
		return
	}

	result, err := c.Comment.ListComments(postID)
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ReactToCommentJSON handles POST /api/v1/comments/{comment_id}/reactions.
func (c CommentController) ReactToCommentJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	req, err := validators.BindCommentReactionRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "malformed request body", nil))
		return
	}

	commentID, err := strconv.ParseInt(r.PathValue("comment_id"), 10, 64)
	if err != nil || commentID <= 0 {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid comment id", nil))
		return
	}
	req.CommentID = commentID

	if details := validators.ValidateCommentReactionRequest(req); details != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid reaction payload", details))
		return
	}

	result, err := c.Comment.ReactToComment(r, req)
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CommentController is the HTTP transport for the comment usecase.
type CommentController struct {
	Comment *usecase.CommentUsecase
}

func NewCommentController(uc *usecase.CommentUsecase) CommentController {
	return CommentController{Comment: uc}
}

// CommentCollectionJSON dispatches POST (create) and GET (list) on the same
// collection path, since ServeMux registers one handler per pattern.
func (c CommentController) CommentCollectionJSON(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		c.CreateCommentJSON(w, r)
	case http.MethodGet:
		c.ListCommentsJSON(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
	}
}

// RegisterCommentRoutes mounts the JSON comment endpoints on the given mux.
func RegisterCommentRoutes(mux *http.ServeMux, c CommentController) {
	mux.HandleFunc("/api/v1/posts/{post_id}/comments", c.CommentCollectionJSON)
	mux.HandleFunc("/api/v1/comments/{comment_id}/reactions", c.ReactToCommentJSON)
}
