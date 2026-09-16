package post

import (
	"forum/server/comment"
	"time"
)

type Post struct {
	ID            int
	UserID        int
	UserName      string
	Title         string
	Content       string

	Likes         int
	Dislikes      int
	Comments      int
	CategoriesStr string
	Categories    []string

	CreatedAt     time.Time

	ImagePath     string
}

type PostDetail struct {
	Post     Post
	Comments []comment.Comment
}
