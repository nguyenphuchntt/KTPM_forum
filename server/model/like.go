package model

import (
	"time"
)

type Like struct {
	UserID    AccountID
	TargetID  int64
	CreatedAt time.Time
}
