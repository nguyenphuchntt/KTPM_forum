package model 

import (
	"time"
)

type PostCategory string
type long int64
type PostID long

const (
	EDUCATION PostCategory = "education_post"
	ENTERTAINMENT PostCategory = "entertainment_post"
)

type Post struct {

	PostID        PostID
	OwnerID       AccountID

	Title         string
	Content       string
	Categories    []PostCategory

	LikeCount         int
	DislikeCount      int
	CommentCount      int

	CreatedAt     time.Time

	MediaID     MediaID
}