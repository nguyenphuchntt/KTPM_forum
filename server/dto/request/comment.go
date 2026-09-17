package request

// CreateCommentRequest is the payload for creating a comment on a post.
type CreateCommentRequest struct {
	PostID  int64  `json:"post_id"`
	Content string `json:"content"`
}

// CommentReactionRequest is the payload for reacting to a comment.
type CommentReactionRequest struct {
	CommentID int64  `json:"comment_id"`
	Reaction  string `json:"reaction"` // "like" or "dislike"
}
