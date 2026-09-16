package model

import "time"

type CommentID int64

type Comment struct {
	ID              CommentID
	UserID          AccountID
	PostID          PostID
	ParentCommentID *CommentID
	Content         string
	CreatedAt       time.Time
}
