package analysis

import "time"

type AnalyzeLastSubmissionResult struct {
	ID           string    `json:"id,omitempty"`
	MatchID      string    `json:"match_id"`
	UserID       string    `json:"user_id"`
	SubmissionID string    `json:"submission_id"`
	Language     string    `json:"language"`
	Code         string    `json:"code,omitempty"`
	PassedCount  int       `json:"passed_count"`
	TotalCount   int       `json:"total_count"`
	Summary      string    `json:"summary"`
	Strengths    []string  `json:"strengths,omitempty"`
	Issues       []string  `json:"issues,omitempty"`
	Suggestions  []string  `json:"suggestions,omitempty"`
	Score        *float64  `json:"score,omitempty"`
	AnalyzedAt   time.Time `json:"analyzed_at"`
	Cached       bool      `json:"cached"`
}

type AnalyzerInput struct {
	ProblemTitle       string `json:"problem_title"`
	ProblemDescription string `json:"problem_description"`
	Language           string `json:"language"`
	Code               string `json:"code"`
	PassedCount        int    `json:"passed_count"`
	TotalCount         int    `json:"total_count"`
}

type HistoryPage struct {
	Items  []AnalyzeLastSubmissionResult `json:"items"`
	Total  int64                         `json:"total"`
	Limit  int                           `json:"limit"`
	Offset int                           `json:"offset"`
}
