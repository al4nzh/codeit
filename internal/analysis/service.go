package analysis

import (
	"context"
	"errors"
	"strings"
	"time"

	"codeit/internal/matches"
	"codeit/internal/problems"
	"codeit/internal/submissions"
)

var (
	ErrInvalidInput         = errors.New("invalid input")
	ErrUnauthorizedForMatch = errors.New("user is not a participant of this match")
	ErrNoSubmissionsYet     = errors.New("no submissions found for this user in this match")
)

type MatchService interface {
	GetByID(ctx context.Context, id string) (*matches.Match, error)
}

type ProblemService interface {
	GetProblemByID(ctx context.Context, id int64) (*problems.ProblemResponse, error)
}

type SubmissionRepository interface {
	GetLastSubmissionByMatchAndUser(ctx context.Context, matchID, userID string) (*submissions.Submission, error)
}

type Service struct {
	matchService MatchService
	problemSvc   ProblemService
	subRepo      SubmissionRepository
	repo         *Repository
	analyzer     AnalyzerClient
}

func NewService(matchService MatchService, problemSvc ProblemService, subRepo SubmissionRepository, repo *Repository, analyzer AnalyzerClient) *Service {
	return &Service{
		matchService: matchService,
		problemSvc:   problemSvc,
		subRepo:      subRepo,
		repo:         repo,
		analyzer:     analyzer,
	}
}

func (s *Service) AnalyzeLastSubmission(ctx context.Context, matchID, userID string, refresh bool) (*AnalyzeLastSubmissionResult, error) {
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

	last, err := s.subRepo.GetLastSubmissionByMatchAndUser(ctx, matchID, userID)
	if err != nil {
		return nil, err
	}
	submissionID := ""
	language := ""
	code := ""
	passedCount := 0
	totalCount := 0
	if last != nil {
		submissionID = last.ID
		language = last.Language
		code = last.Code
		passedCount = last.PassedCount
		totalCount = last.TotalCount
	}
	if !refresh && s.repo != nil {
		existing, err := s.repo.GetLatestByMatchAndUser(ctx, matchID, userID)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.SubmissionID == submissionID {
			existing.Cached = true
			return existing, nil
		}
	}

	problem, err := s.problemSvc.GetProblemByID(ctx, match.ProblemID)
	if err != nil {
		return nil, err
	}

	out, err := s.analyzer.Analyze(ctx, AnalyzerInput{
		ProblemTitle:       problem.Title,
		ProblemDescription: problem.Description,
		Language:           language,
		Code:               code,
		PassedCount:        passedCount,
		TotalCount:         totalCount,
	})
	if err != nil {
		return nil, err
	}

	out.MatchID = matchID
	out.UserID = userID
	out.SubmissionID = submissionID
	out.Language = language
	out.Code = code
	out.PassedCount = passedCount
	out.TotalCount = totalCount
	out.AnalyzedAt = time.Now()
	out.Cached = false
	if s.repo != nil {
		saved, err := s.repo.Save(ctx, out)
		if err != nil {
			return nil, err
		}
		return saved, nil
	}
	return out, nil
}

func (s *Service) GetLatestMatchAnalysis(ctx context.Context, matchID, userID string) (*AnalyzeLastSubmissionResult, error) {
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
	if s.repo == nil {
		return nil, ErrAnalyzerUnavailable
	}
	res, err := s.repo.GetLatestByMatchAndUser(ctx, matchID, userID)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrNoSubmissionsYet
	}
	res.Cached = true
	return res, nil
}

func (s *Service) ListMyAnalyses(ctx context.Context, userID string, limit, offset int) (*HistoryPage, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidInput
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	if s.repo == nil {
		return nil, ErrAnalyzerUnavailable
	}
	return s.repo.ListByUser(ctx, userID, limit, offset)
}
