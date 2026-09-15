package model

import "time"

type MediaID long

type Media struct {
	ID        MediaID

	ObjectKey string
	PublicURL string
	MIMEType  string
	Size      long

	CreatedAt time.Time
	UpdatedAt time.Time
}
