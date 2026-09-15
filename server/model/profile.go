package model

import (
	"time"
)

type Profile struct {
	firstName string 
	lastName string 

	location string

	avatarMediaId MediaID
	coverMediaId MediaID

	createdAt time.Time
	updatedAt time.Time
}