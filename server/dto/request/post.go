package request

type CreatePostRequest struct {
	Title      string `json:"title"`
	Content    string `json:"content"`
	Categories []int  `json:"categories"`
	MediaID    *int64 `json:"media_id,omitempty"`
}

type ReactToPostRequest struct {
	PostID   int64  `json:"post_id,omitempty"`
	Reaction string `json:"reaction"`
}
