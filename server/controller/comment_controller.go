package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"forum/server/dto/request"
	"forum/server/dto/response"
	"forum/server/logger"
	"forum/server/usecase"
	"forum/server/validators"
)

// CreateCommentSSR handles legacy SSR POST /post/addcommentREQ by delegating to CommentUsecase.
func (c CommentController) CreateCommentSSR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	postID, err := strconv.ParseInt(r.FormValue("postid"), 10, 64)
	if err != nil || postID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := request.CreateCommentRequest{
		PostID:  postID,
		Content: r.FormValue("comment"),
	}

	res, err := c.Comment.CreateComment(r, req)
	if err != nil {
		writeAppError(w, err)
		return
	}

	resp := map[string]any{
		"ID":            res.Comment.ID,
		"username":      res.Comment.Username,
		"created_at":    res.Comment.CreatedAt.Format("01/02/2006 03:04 PM"),
		"content":       res.Comment.Content,
		"likes":         res.Comment.Likes,
		"dislikes":      res.Comment.Dislikes,
		"commentscount": res.CommentsCount,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// ReactToCommentSSR handles legacy SSR POST /post/commentreaction by delegating to CommentUsecase.
func (c CommentController) ReactToCommentSSR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	commentID, err := strconv.ParseInt(r.FormValue("comment_id"), 10, 64)
	if err != nil || commentID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req := request.CommentReactionRequest{
		CommentID: commentID,
		Reaction:  r.FormValue("reaction"),
	}

	res, err := c.Comment.ReactToComment(r, req)
	if err != nil {
		writeAppError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]int{
		"commentlikesCount":    res.Likes,
		"commentdislikesCount": res.Dislikes,
	})
}

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
