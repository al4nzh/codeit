package friendbattles

import "time"

const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
)

type Invite struct {
	ID               string
	Code             string
	HostUserID       string
	Status           string
	Difficulty       string
	DurationSeconds  int
	SkipElo          bool
	MatchID          *string
	ExpiresAt        time.Time
	CreatedAt        time.Time
}
