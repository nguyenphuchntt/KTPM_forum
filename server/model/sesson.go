package model

import (
	"time"

	"github.com/google/uuid"
)

type SessionID uuid.UUID

type Session struct {
	ID SessionID

	OwnerID AccountID

	ExpiresAt time.Time
}