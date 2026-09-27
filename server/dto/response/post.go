package response

import (
	"time"

	"github.com/google/uuid"

	"forum/server/model"
)

type PostResponse struct {
	ID           int64                `json:"id"`
	UserID       string               `json:"user_id"`
	Title        string               `json:"title"`
	Content      string               `json:"content"`
	MediaID      *int64               `json:"media_id,omitempty"`
	LikeCount    int                  `json:"like_count"`
	DislikeCount int                  `json:"dislike_count"`
	CommentCount int                  `json:"comment_count"`
	CreatedAt    time.Time            `json:"created_at"`
	Categories   []model.PostCategory `json:"categories,omitempty"`
}

type PostListResponse struct {
	Data []PostResponse `json:"data"`
}

// SearchPostItem is a post plus how well it matched. TitleSnippet and Snippet
// hold the title and a content excerpt with the query terms wrapped in <mark>;
// the plain Title and Content are still sent alongside them.
type SearchPostItem struct {
	PostResponse
	Rank         float32 `json:"rank"`
	TitleSnippet string  `json:"title_snippet"`
	Snippet      string  `json:"snippet"`
}

type SearchPostListResponse struct {
	Data       []SearchPostItem `json:"data"`
	TotalCount int              `json:"total_count"`
	Offset     int              `json:"offset"`
}

type ReactToPostResponse struct {
	Likes    int `json:"likes"`
	Dislikes int `json:"dislikes"`
}

func NewPostResponse(p model.Post) PostResponse {
	var mediaID *int64
	if p.MediaID != nil {
		id := int64(*p.MediaID)
		mediaID = &id
	}
	return PostResponse{
		ID:           int64(p.ID),
		UserID:       uuid.UUID(p.UserID).String(),
		Title:        p.Title,
		Content:      p.Content,
		MediaID:      mediaID,
		LikeCount:    p.LikeCount,
		DislikeCount: p.DislikeCount,
		CommentCount: p.CommentCount,
		CreatedAt:    p.CreatedAt,
		Categories:   p.Categories,
	}
}

func NewPostListResponse(posts []model.Post) PostListResponse {
	res := make([]PostResponse, 0, len(posts))
	for _, p := range posts {
		res = append(res, NewPostResponse(p))
	}
	return PostListResponse{Data: res}
}
