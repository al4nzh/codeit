package submissions

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"codeit/internal/matches"
	"codeit/internal/problems"
	"github.com/google/uuid"
)

var (
	ErrInvalidInput         = errors.New("invalid input")
	ErrUnauthorizedForMatch = errors.New("user is not a participant of this match")
	ErrMatchNotRunning      = errors.New("match is not running")
	ErrMatchExpired         = errors.New("match has expired")
	ErrMatchNotExpiredYet   = errors.New("match has not expired yet")
)

type MatchService interface {
	GetByID(ctx context.Context, id string) (*matches.Match, error)
	FinishMatch(ctx context.Context, id string, winnerID string) error
	FinishMatchWithVictoryType(ctx context.Context, id string, winnerID, victoryType string) error
}

type ProblemService interface {
	GetAllTestCasesForJudge(ctx context.Context, problemID int64) ([]problems.TestCase, error)
}

// RatingService updates user ratings after a match is finished (optional for tests).
type RatingService interface {
	ApplyFinishedMatch(ctx context.Context, match *matches.Match) error
}

type Service struct {
	repo           *SubmissionRepository
	matchService   MatchService
	problemService ProblemService
	judge          Judge
	ratings        RatingService
}

func NewService(repo *SubmissionRepository, matchService MatchService, problemService ProblemService, judge Judge, ratings RatingService) *Service {
	return &Service{
		repo:           repo,
		matchService:   matchService,
		problemService: problemService,
		judge:          judge,
		ratings:        ratings,
	}
}

func (s *Service) Submit(ctx context.Context, matchID, userID, language, code string) (*SubmitResult, error) {
	matchID = strings.TrimSpace(matchID)
	userID = strings.TrimSpace(userID)
	language = strings.TrimSpace(strings.ToLower(language))
	code = strings.TrimSpace(code)

	if matchID == "" || userID == "" || language == "" || code == "" {
		return nil, ErrInvalidInput
	}

	match, err := s.matchService.GetByID(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if match.Player1ID != userID && match.Player2ID != userID {
		return nil, ErrUnauthorizedForMatch
	}
	if match.Status != matches.StatusRunning {
		return nil, ErrMatchNotRunning
	}
	if match.StartedAt == nil || match.DurationSeconds <= 0 {
		return nil, ErrInvalidInput
	}

	testCases, err := s.problemService.GetAllTestCasesForJudge(ctx, match.ProblemID)
	if err != nil {
		return nil, err
	}
	totalCount := len(testCases)
	passedCount := 0
	for _, tc := range testCases {
		passed, err := s.judge.Evaluate(ctx, language, code, tc.Input, tc.Expected)
		if err != nil {
			return nil, err
		}
		if passed {
			passedCount++
		}
	}

	submission := &Submission{
		ID:          uuid.New().String(),
		MatchID:     matchID,
		UserID:      userID,
		Language:    language,
		Code:        code,
		PassedCount: passedCount,
		TotalCount:  totalCount,
		Status:      StatusJudged,
		SubmittedAt: time.Now(),
	}
	if err := s.repo.CreateSubmission(ctx, submission); err != nil {
		return nil, err
	}

	result := &SubmitResult{Submission: submission}
	deadline := match.StartedAt.Add(time.Duration(match.DurationSeconds) * time.Second)
	now := time.Now()
	if now.After(deadline) {
		if err := s.finishByBestScore(ctx, match); err != nil {
			return nil, err
		}
		updatedMatch, err := s.matchService.GetByID(ctx, match.ID)
		if err != nil {
			return nil, err
		}
		result.MatchFinished = true
		result.MatchResult = updatedMatch.Result
		s.applyRatings(ctx, updatedMatch)
		return result, nil
	}

	// MVP: full-score submission wins immediately.
	if totalCount > 0 && passedCount == totalCount {
		if err := s.matchService.FinishMatchWithVictoryType(ctx, match.ID, userID, matches.VictoryTypeKO); err != nil {
			return nil, err
		}
		updatedMatch, err := s.matchService.GetByID(ctx, match.ID)
		if err != nil {
			return nil, err
		}
		result.MatchFinished = true
		result.MatchResult = updatedMatch.Result
		s.applyRatings(ctx, updatedMatch)
		return result, nil
	}

	return result, nil
}

func (s *Service) finishByBestScore(ctx context.Context, match *matches.Match) error {
	player1Best, err := s.repo.GetBestPassedCountByMatchAndUser(ctx, match.ID, match.Player1ID)
	if err != nil {
		return err
	}
	player2Best, err := s.repo.GetBestPassedCountByMatchAndUser(ctx, match.ID, match.Player2ID)
	if err != nil {
		return err
	}

	if player1Best == player2Best {
		return s.matchService.FinishMatchWithVictoryType(ctx, match.ID, "", matches.VictoryTypeDraw)
	}
	if player1Best > player2Best {
		return s.matchService.FinishMatchWithVictoryType(ctx, match.ID, match.Player1ID, matches.VictoryTypeDecision)
	}
	return s.matchService.FinishMatchWithVictoryType(ctx, match.ID, match.Player2ID, matches.VictoryTypeDecision)
}

func (s *Service) applyRatings(ctx context.Context, finishedMatch *matches.Match) {
	if s.ratings == nil || finishedMatch == nil {
		return
	}
	if err := s.ratings.ApplyFinishedMatch(ctx, finishedMatch); err != nil {
		log.Printf("ratings: apply after match %s: %v", finishedMatch.ID, err)
	}
}

// ResolveMatchOutcome is returned by ResolveExpiredMatch.
type ResolveMatchOutcome struct {
	Match           *matches.Match `json:"match"`
	Resolved        bool           `json:"resolved"`         // true if this call finished the match now
	AlreadyFinished bool           `json:"already_finished"` // true if match was already finished
}

// ResolveExpiredMatch finishes a running match after its duration has passed, using best
// passed_count per player (same rules as post-deadline submission). Idempotent if already finished.
func (s *Service) ResolveExpiredMatch(ctx context.Context, matchID, userID string) (*ResolveMatchOutcome, error) {
	matchID = strings.TrimSpace(matchID)
	userID = strings.TrimSpace(userID)
	if matchID == "" || userID == "" {
		return nil, ErrInvalidInput
	}

	match, err := s.matchService.GetByID(ctx, matchID)
	if err != nil {
		return nil, err
	}
	if match.Player1ID != userID && match.Player2ID != userID {
		return nil, ErrUnauthorizedForMatch
	}

	if match.Status == matches.StatusFinished {
		return &ResolveMatchOutcome{Match: match, Resolved: false, AlreadyFinished: true}, nil
	}
	if match.Status != matches.StatusRunning {
		return nil, ErrMatchNotRunning
	}
	if match.StartedAt == nil || match.DurationSeconds <= 0 {
		return nil, ErrInvalidInput
	}

	deadline := match.StartedAt.Add(time.Duration(match.DurationSeconds) * time.Second)
	if time.Now().Before(deadline) {
		return nil, ErrMatchNotExpiredYet
	}

	if err := s.finishByBestScore(ctx, match); err != nil {
		updated, gerr := s.matchService.GetByID(ctx, matchID)
		if gerr != nil {
			return nil, err
		}
		if updated.Status == matches.StatusFinished {
			return &ResolveMatchOutcome{Match: updated, Resolved: false, AlreadyFinished: true}, nil
		}
		return nil, err
	}

	updatedMatch, err := s.matchService.GetByID(ctx, match.ID)
	if err != nil {
		return nil, err
	}
	s.applyRatings(ctx, updatedMatch)
	return &ResolveMatchOutcome{Match: updatedMatch, Resolved: true, AlreadyFinished: false}, nil
}
