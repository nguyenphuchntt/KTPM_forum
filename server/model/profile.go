package model

import "time"

type Profile struct {
	AccountID     AccountID
	FirstName     string
	LastName      string
	Location      string
	AvatarMediaID *MediaID
	CoverMediaID  *MediaID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
