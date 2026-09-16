package model

import (
	"time"

	"github.com/google/uuid"
)

type AccountID uuid.UUID

type Account struct {
	ID        AccountID
	Email     string
	Username  string
	Password  string
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
