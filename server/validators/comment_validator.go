package validators

import (
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"

	"forum/server/dto/request"
)

// BindCreateCommentRequest decodes a comment creation payload from JSON or form.
func BindCreateCommentRequest(r *http.Request) (request.CreateCommentRequest, error) {
	var req request.CreateCommentRequest

	if isJSON(r) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return req, err
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return req, err
		}
		postIDStr := r.FormValue("postid")
		if postIDStr == "" {
			postIDStr = r.FormValue("post_id")
		}
		postID, _ := strconv.ParseInt(postIDStr, 10, 64)
		req.PostID = postID
		req.Content = r.FormValue("comment")
		if req.Content == "" {
			req.Content = r.FormValue("content")
		}
	}

	req.Content = strings.TrimSpace(req.Content)

	return req, nil
}

// ValidateCreateCommentRequest returns per-field errors or nil.
func ValidateCreateCommentRequest(req request.CreateCommentRequest) map[string]string {
	details := map[string]string{}

	if req.PostID <= 0 {
		details["post_id"] = "post_id must be a positive integer"
	}

	// Escape content để check length chính xác như usecase sẽ lưu
	escapedContent := html.EscapeString(req.Content)
	switch {
	case escapedContent == "":
		details["content"] = "content is required"
	case len(escapedContent) > 1800:
		details["content"] = "content exceeds maximum length of 1800 characters"
	}

	if len(details) == 0 {
		return nil
	}
	return details
}

// BindCommentReactionRequest decodes a comment reaction payload from JSON or form.
func BindCommentReactionRequest(r *http.Request) (request.CommentReactionRequest, error) {
	var req request.CommentReactionRequest

	if isJSON(r) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return req, err
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return req, err
		}
		commentIDStr := r.FormValue("comment_id")
		commentID, _ := strconv.ParseInt(commentIDStr, 10, 64)
		req.CommentID = commentID
		req.Reaction = r.FormValue("reaction")
	}

	req.Reaction = strings.ToLower(strings.TrimSpace(req.Reaction))

	return req, nil
}

// ValidateCommentReactionRequest returns per-field errors or nil.
func ValidateCommentReactionRequest(req request.CommentReactionRequest) map[string]string {
	details := map[string]string{}

	if req.CommentID <= 0 {
		details["comment_id"] = "comment_id must be a positive integer"
	}

	if req.Reaction != "like" && req.Reaction != "dislike" {
		details["reaction"] = "reaction must be 'like' or 'dislike'"
	}

	if len(details) == 0 {
		return nil
	}
	return details
}
