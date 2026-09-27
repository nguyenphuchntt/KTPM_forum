package model

import "time"

// LikeTargetType distinguishes what a row in `likes` points at, because post ids
// and comment ids come from different tables and can collide.
type LikeTargetType string

const (
	LikeTargetPost    LikeTargetType = "post"
	LikeTargetComment LikeTargetType = "comment"
)

// LikeReaction is the kind of reaction a user applied to a target.
type LikeReaction string

const (
	ReactionLike    LikeReaction = "like"
	ReactionDislike LikeReaction = "dislike"
)

type Like struct {
	UserID     AccountID
	TargetType LikeTargetType
	TargetID   int64
	Reaction   LikeReaction
	CreatedAt  time.Time
}
