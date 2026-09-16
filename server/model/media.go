package model

import "time"

type MediaID int64

type Media struct {
	ID        MediaID
	ObjectKey string
	PublicURL string
	MIMEType  string
	Size      int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
