package model

import "time"

type PostID int64
type CategoryID int64

type PostCategory struct {
	ID         int64
	PostID     PostID
	CategoryID CategoryID
	Label      string
}

type Post struct {
	ID         PostID
	UserID     AccountID
	Title      string
	Content    string
	MediaID    *MediaID
	CreatedAt  time.Time
	Categories []PostCategory
}
