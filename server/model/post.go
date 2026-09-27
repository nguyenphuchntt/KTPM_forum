package model

import (
	"fmt"
	"time"
)

type PostID int64
type CategoryID int64

type PostCategory struct {
	ID         int64
	PostID     PostID
	CategoryID CategoryID
	Label      string
}

type Post struct {
	ID           PostID
	UserID       AccountID
	Username     string
	Title        string
	Content      string
	MediaID      *MediaID
	LikeCount    int
	DislikeCount int
	CommentCount int
	CreatedAt    time.Time
	Categories   []PostCategory
}

func (p Post) UserName() string {
	return p.Username
}

func (p Post) Likes() int {
	return p.LikeCount
}

func (p Post) Dislikes() int {
	return p.DislikeCount
}

func (p Post) Comments() int {
	return p.CommentCount
}

func (p Post) ImagePath() string {
	if p.MediaID == nil {
		return ""
	}
	return fmt.Sprintf("/api/v1/medias/%d", *p.MediaID)
}
