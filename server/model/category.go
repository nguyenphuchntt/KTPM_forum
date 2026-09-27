package model

import "time"

type Category struct {
	ID        CategoryID
	Label     string
	CreatedAt time.Time
}
