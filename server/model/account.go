package model

import (
	"time"

	"github.com/google/uuid"
)

type AccountID uuid.UUID

type Account struct {
	ID AccountID

	Username string 
	Password string 

	Email string

	Role string 

	CreatedAt time.Time
	UpdatedAt time.Time
}