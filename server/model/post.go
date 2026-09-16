package model 

import (
	"time"
)

type PostCategory struct {
	ID int
	label string
}

type long int64
type PostID long

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