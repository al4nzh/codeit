package submissions

import "time"

const (
	StatusJudged = "judged"
)

type Submission struct {
	ID          string    `json:"id"`
	MatchID     string    `json:"match_id"`
	UserID      string    `json:"user_id"`
	Language    string    `json:"language"`
	Code        string    `json:"code,omitempty"`
	PassedCount int       `json:"passed_count"`
	TotalCount  int       `json:"total_count"`
	Status      string    `json:"status"`
	SubmittedAt time.Time `json:"submitted_at"`
}

type SubmitResult struct {
	Submission    *Submission `json:"submission"`
	MatchFinished bool        `json:"match_finished"`
	MatchResult   string      `json:"match_result,omitempty"`
}
