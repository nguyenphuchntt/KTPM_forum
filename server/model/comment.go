package model

import "time"

type CommentID int64

type Comment struct {
	ID              CommentID
	UserID          AccountID
	Username        string
	PostID          PostID
	ParentCommentID *CommentID
	Content         string
	LikeCount       int
	DislikeCount    int
	CreatedAt       time.Time
}

func (c Comment) UserName() string {
	return c.Username
}

func (c Comment) Likes() int {
	return c.LikeCount
}

func (c Comment) Dislikes() int {
	return c.DislikeCount
}
