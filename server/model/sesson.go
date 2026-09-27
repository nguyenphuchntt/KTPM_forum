package model

import "time"

type Session struct {
	UserID    AccountID
	SessionID string
	ExpiresAt time.Time
}
