package model

import (
	"time"
	"github.com/google/uuid"
)

type Like struct {
	UserID    uuid.UUID
	TargetID    long
	CreatedAt time.Time
}
