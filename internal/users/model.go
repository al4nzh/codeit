package users

import "time"

type User struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Password    string    `json:"password"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Rating      int       `json:"rating"`
	CreatedAt   time.Time `json:"created_at"`
	WorldRank   int       `json:"world_rank,omitempty"`
	RatingTitle string    `json:"rating_title,omitempty"`
}

type LeaderboardEntry struct {
	WorldRank int    `json:"world_rank"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Rating    int    `json:"rating"`
	Title     string `json:"title"`
}