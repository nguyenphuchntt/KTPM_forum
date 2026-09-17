package response

import "time"

// CommentResponse represents a single comment returned by the API.
type CommentResponse struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Likes     int       `json:"likes"`
	Dislikes  int       `json:"dislikes"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateCommentResponse is returned after successfully creating a comment.
type CreateCommentResponse struct {
	Comment       CommentResponse `json:"comment"`
	CommentsCount int             `json:"comments_count"`
}

// CommentListResponse is the envelope for GET /api/v1/posts/{post_id}/comments.
type CommentListResponse struct {
	Data []CommentResponse `json:"data"`
}

// CommentReactionResponse is returned after reacting to a comment.
type CommentReactionResponse struct {
	CommentID int `json:"comment_id"`
	Likes     int `json:"likes"`
	Dislikes  int `json:"dislikes"`
}
