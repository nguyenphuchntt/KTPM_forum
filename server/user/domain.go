package user

import (
	"time"
)

type User struct {
	username string 
	password string 

	email string

	role string 

	createdAt time.Time
	updatedAt time.Time
}