package model

import (
	"time"
)

type CommentID long

type Comment struct {
	ID        CommentID
	OwnerID    AccountID

	ParentCommentID CommentID
	PostID    PostID

	Content   string

	LikeCount     int
	DislikeCount  int

	CreatedAt time.Time
}
