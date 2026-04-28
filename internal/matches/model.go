package matches

import "time"

const (
	StatusWaiting  = "waiting"
	StatusRunning  = "running"
	StatusFinished = "finished"
)

const (
	ResultPending = "pending"
	ResultPlayer1 = "player1"
	ResultPlayer2 = "player2"
	ResultDraw    = "draw"
)

// Per-viewer labels for finished matches (match history).
const (
	MyResultWin  = "win"
	MyResultLoss = "loss"
	MyResultDraw = "draw"
)

// UserMatchStats are aggregates over finished matches for one user.
type UserMatchStats struct {
	MatchesPlayed int `json:"matches_played"`
	Wins          int `json:"wins"`
	Losses        int `json:"losses"`
	Draws         int `json:"draws"`
}

// MatchHistoryEntry is one finished match from the perspective of the requesting user.
type MatchHistoryEntry struct {
	Match         Match  `json:"match"`
	OpponentID    string `json:"opponent_id"`
	MyResult      string `json:"my_result"`                 // MyResultWin | MyResultLoss | MyResultDraw
	MyRatingAfter *int   `json:"my_rating_after,omitempty"` // rating after this match, when snapshot exists
	MyEloDelta    *int   `json:"my_elo_delta,omitempty"`    // change from this match, when snapshot exists
}

// MatchHistoryPage is a paginated list of finished matches for a user.
type MatchHistoryPage struct {
	Matches []MatchHistoryEntry `json:"matches"`
	Total   int64               `json:"total"`
	Limit   int                 `json:"limit"`
	Offset  int                 `json:"offset"`
}

type Match struct {
	ID                 string     `json:"id"`
	Player1ID          string     `json:"player1_id"`
	Player2ID          string     `json:"player2_id"`
	ProblemID          int64      `json:"problem_id"`
	Status             string     `json:"status"`
	DurationSeconds    int        `json:"duration_seconds"`
	Result             string     `json:"result,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	WinnerID           *string    `json:"winner_id,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	Player1RatingAfter *int       `json:"player1_rating_after,omitempty"`
	Player2RatingAfter *int       `json:"player2_rating_after,omitempty"`
	Player1EloDelta    *int       `json:"player1_elo_delta,omitempty"`
	Player2EloDelta    *int       `json:"player2_elo_delta,omitempty"`
}

func ResolveMatchResult(match *Match) string {
	if match == nil {
		return ResultPending
	}
	if match.Status != StatusFinished {
		return ResultPending
	}
	if match.WinnerID == nil {
		return ResultDraw
	}
	if *match.WinnerID == match.Player1ID {
		return ResultPlayer1
	}
	if *match.WinnerID == match.Player2ID {
		return ResultPlayer2
	}
	// Defensive fallback for inconsistent historical rows.
	return ResultPending
}

// OpponentID returns the other player; empty if userID is not a participant.
func OpponentID(match *Match, userID string) string {
	if match == nil || userID == "" {
		return ""
	}
	if match.Player1ID == userID {
		return match.Player2ID
	}
	if match.Player2ID == userID {
		return match.Player1ID
	}
	return ""
}

// MyResultForUser maps a finished match to win/loss/draw for the given user.
func MyResultForUser(match *Match, userID string) string {
	if match == nil || userID == "" || match.Status != StatusFinished {
		return ""
	}
	if match.WinnerID == nil {
		return MyResultDraw
	}
	if *match.WinnerID == userID {
		return MyResultWin
	}
	return MyResultLoss
}

// MyRatingAfterForUser returns the stored post-match rating for this user, if present.
func MyRatingAfterForUser(m *Match, userID string) *int {
	if m == nil || userID == "" {
		return nil
	}
	if m.Player1ID == userID {
		return m.Player1RatingAfter
	}
	if m.Player2ID == userID {
		return m.Player2RatingAfter
	}
	return nil
}

// MyEloDeltaForUser returns the stored Elo change for this user for this match, if present.
func MyEloDeltaForUser(m *Match, userID string) *int {
	if m == nil || userID == "" {
		return nil
	}
	if m.Player1ID == userID {
		return m.Player1EloDelta
	}
	if m.Player2ID == userID {
		return m.Player2EloDelta
	}
	return nil
}
