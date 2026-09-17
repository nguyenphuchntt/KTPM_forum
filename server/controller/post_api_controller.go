package controllers

import (
	"net/http"
	"strconv"

	"forum/server/dto/response"
	"forum/server/logger"
	"forum/server/model"
	"forum/server/usecase"
	"forum/server/validators"
)

// PostsCollectionJSON dispatches GET (list) and POST (create) for /api/v1/posts.
func (c PostController) PostsCollectionJSON(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.ListPostsJSON(w, r)
	case http.MethodPost:
		c.CreatePostJSON(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
	}
}

// PostItemJSON dispatches GET (detail) and DELETE (delete) for /api/v1/posts/{id}.
func (c PostController) PostItemJSON(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.ShowPostJSON(w, r)
	case http.MethodDelete:
		c.DeletePostJSON(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
	}
}

// ListPostsJSON handles GET /api/v1/posts.
func (c PostController) ListPostsJSON(w http.ResponseWriter, r *http.Request) {
	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	catIDStr := r.URL.Query().Get("category_id")
	var posts []model.Post
	var status int
	var err error

	if catIDStr != "" {
		catID, parseErr := strconv.Atoi(catIDStr)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest,
				response.NewError(response.CodeValidationError, "invalid category_id", nil))
			return
		}
		posts, status, err = c.Post.ListPostsByCategory(catID, offset)
	} else {
		posts, status, err = c.Post.ListPosts(offset)
	}

	if err != nil {
		writeAppError(w, err)
		return
	}
	if status != http.StatusOK {
		writeJSON(w, status, response.NewError(response.CodeValidationError, err.Error(), nil))
		return
	}

	writeJSON(w, http.StatusOK, response.NewPostListResponse(posts))
}

// ShowPostJSON handles GET /api/v1/posts/{id}.
func (c PostController) ShowPostJSON(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || postID <= 0 {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid post id", nil))
		return
	}

	detail, status, err := c.Post.FetchPost(postID)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if status != http.StatusOK {
		writeJSON(w, status, response.NewError(response.CodeValidationError, err.Error(), nil))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"post":     response.NewPostResponse(detail.Post),
		"comments": detail.Comments,
	})
}

// CreatePostJSON handles POST /api/v1/posts.
func (c PostController) CreatePostJSON(w http.ResponseWriter, r *http.Request) {
	req, err := validators.BindCreatePostRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "malformed request body", nil))
		return
	}

	if details := validators.ValidateCreatePostRequest(req); details != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid post payload", details))
		return
	}

	catStrings := make([]string, 0, len(req.Categories))
	for _, id := range req.Categories {
		catStrings = append(catStrings, strconv.Itoa(id))
	}

	input := usecase.CreatePostInput{
		Request:    r,
		Title:      req.Title,
		Content:    req.Content,
		Categories: catStrings,
	}

	res, err := c.Post.CreatePost(input)
	if err != nil {
		writeAppError(w, err)
		return
	}

	log := logger.WithRequest(r, "")
	log.Info().Int64("post_id", res.PostID).Msg("Post created via JSON API")

	writeJSON(w, http.StatusCreated, res)
}

// DeletePostJSON handles DELETE /api/v1/posts/{id}.
func (c PostController) DeletePostJSON(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || postID <= 0 {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid post id", nil))
		return
	}

	input := usecase.DeletePostInput{
		Request: r,
		PostID:  postID,
	}

	res, status, err := c.Post.DeletePost(input)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if status != http.StatusOK {
		writeJSON(w, status, response.NewError(response.CodeValidationError, err.Error(), nil))
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// ReactToPostJSON handles POST /api/v1/posts/{id}/reactions.
func (c PostController) ReactToPostJSON(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || postID <= 0 {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid post id", nil))
		return
	}

	req, err := validators.BindReactToPostRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "malformed request body", nil))
		return
	}
	req.PostID = postID

	if details := validators.ValidateReactToPostRequest(req); details != nil {
		writeJSON(w, http.StatusBadRequest,
			response.NewError(response.CodeValidationError, "invalid reaction payload", details))
		return
	}

	input := usecase.ReactToPostInput{
		Request:  r,
		PostID:   postID,
		Reaction: req.Reaction,
	}

	res, err := c.Post.ReactToPost(input)
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response.ReactToPostResponse{
		Likes:    res.Likes,
		Dislikes: res.Dislikes,
	})
}

// MyPostsJSON handles GET /api/v1/me/posts.
func (c PostController) MyPostsJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	session := c.Post.ValidateSession(r)
	if !session.Valid {
		writeJSON(w, http.StatusUnauthorized,
			response.NewError(response.CodeValidationError, "unauthorized", nil))
		return
	}

	posts, status, err := c.Post.FetchCreatedPosts(model.AccountID(session.UserID), offset)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if status != http.StatusOK {
		writeJSON(w, status, response.NewError(response.CodeValidationError, err.Error(), nil))
		return
	}

	writeJSON(w, http.StatusOK, response.NewPostListResponse(posts))
}

// MyLikedPostsJSON handles GET /api/v1/me/liked-posts.
func (c PostController) MyLikedPostsJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed,
			response.NewError(response.CodeValidationError, "method not allowed", nil))
		return
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	session := c.Post.ValidateSession(r)
	if !session.Valid {
		writeJSON(w, http.StatusUnauthorized,
			response.NewError(response.CodeValidationError, "unauthorized", nil))
		return
	}

	posts, status, err := c.Post.FetchLikedPosts(model.AccountID(session.UserID), offset)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if status != http.StatusOK {
		writeJSON(w, status, response.NewError(response.CodeValidationError, err.Error(), nil))
		return
	}

	writeJSON(w, http.StatusOK, response.NewPostListResponse(posts))
}
