package problems

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v4"
)

var ErrInvalidProblemID = errors.New("invalid problem id")
var ErrProblemNotFound = errors.New("problem not found")
var ErrInvalidDifficulty = errors.New("invalid difficulty")

type ProblemService struct {
	repo *ProblemRepository
}

func NewProblemService(repo *ProblemRepository) *ProblemService {
	return &ProblemService{repo: repo}
}

func (s *ProblemService) ListProblems(ctx context.Context) ([]ProblemResponse, error) {
	problems, err := s.repo.ListProblems(ctx)
	if err != nil {
		return nil, err
	}

	responses := make([]ProblemResponse, 0, len(problems))
	for _, problem := range problems {
		sampleCases, err := s.repo.GetSampleTestCasesByProblemID(ctx, problem.ID)
		if err != nil {
			return nil, err
		}
		responses = append(responses, *NewProblemResponse(&problem, sampleCases))
	}

	return responses, nil
}

func (s *ProblemService) GetProblemByID(ctx context.Context, id int64) (*ProblemResponse, error) {
	if id <= 0 {
		return nil, ErrInvalidProblemID
	}

	problem, err := s.repo.GetProblemByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProblemNotFound
		}
		return nil, err
	}

	sampleCases, err := s.repo.GetSampleTestCasesByProblemID(ctx, problem.ID)
	if err != nil {
		return nil, err
	}

	return NewProblemResponse(problem, sampleCases), nil
}

func (s *ProblemService) GetRandomProblemByDifficulty(ctx context.Context, difficulty string) (*ProblemResponse, error) {
	difficulty = strings.TrimSpace(strings.ToLower(difficulty))
	if difficulty == "" {
		return nil, ErrInvalidDifficulty
	}

	problem, err := s.repo.GetRandomProblemByDifficulty(ctx, difficulty)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProblemNotFound
		}
		return nil, err
	}

	sampleCases, err := s.repo.GetSampleTestCasesByProblemID(ctx, problem.ID)
	if err != nil {
		return nil, err
	}

	return NewProblemResponse(problem, sampleCases), nil
}

// GetAllTestCasesForJudge is intentionally backend-only and must not be returned to clients.
func (s *ProblemService) GetAllTestCasesForJudge(ctx context.Context, problemID int64) ([]TestCase, error) {
	if problemID <= 0 {
		return nil, ErrInvalidProblemID
	}

	testCases, err := s.repo.GetAllTestCasesByProblemID(ctx, problemID)
	if err != nil {
		return nil, err
	}
	return testCases, nil
}
