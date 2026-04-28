package matchmaking

import (
	"context"
	"errors"
	"strings"
	"sync"

	"codeit/internal/matches"
	"codeit/internal/problems"
)

var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrInvalidDifficulty = errors.New("invalid difficulty")
	ErrAlreadyInQueue    = errors.New("user is already in matchmaking queue")
	ErrAlreadyInMatch    = errors.New("user already has an active match")
)

type MatchService interface {
	CreateMatch(ctx context.Context, player1ID, player2ID string, problemID int64, durationSeconds int) (*matches.Match, error)
	GetActiveMatchByUserID(ctx context.Context, userID string) (*matches.Match, error)
}

type ProblemService interface {
	GetRandomProblemByDifficulty(ctx context.Context, difficulty string) (*problems.ProblemResponse, error)
}

type Service struct {
	matchService   MatchService
	problemService ProblemService

	mu                  sync.Mutex
	waitingByDifficulty map[string][]string
}

const (
	defaultMatchDurationSeconds = 20 * 60
)

var allowedDifficulties = map[string]struct{}{
	"easy":   {},
	"medium": {},
	"hard":   {},
}

func NewService(matchService MatchService, problemService ProblemService) *Service {
	return &Service{
		matchService:         matchService,
		problemService:       problemService,
		waitingByDifficulty:  make(map[string][]string),
	}
}

func (s *Service) EnqueueOrMatch(ctx context.Context, userID, difficulty string) (*matches.Match, bool, error) {
	userID = strings.TrimSpace(userID)
	difficulty = strings.TrimSpace(strings.ToLower(difficulty))
	if userID == "" || difficulty == "" {
		return nil, false, ErrInvalidInput
	}
	if _, ok := allowedDifficulties[difficulty]; !ok {
		return nil, false, ErrInvalidDifficulty
	}

	_, err := s.matchService.GetActiveMatchByUserID(ctx, userID)
	if err == nil {
		return nil, false, ErrAlreadyInMatch
	}
	if !errors.Is(err, matches.ErrMatchNotFound) {
		return nil, false, err
	}

	s.mu.Lock()
	if s.isQueuedLocked(userID) {
		s.mu.Unlock()
		return nil, false, ErrAlreadyInQueue
	}

	queue := s.waitingByDifficulty[difficulty]
	if len(queue) == 0 {
		s.waitingByDifficulty[difficulty] = append(queue, userID)
		s.mu.Unlock()
		return nil, false, nil
	}

	opponentID := queue[0]
	if len(queue) == 1 {
		delete(s.waitingByDifficulty, difficulty)
	} else {
		s.waitingByDifficulty[difficulty] = queue[1:]
	}
	s.mu.Unlock()

	problem, err := s.problemService.GetRandomProblemByDifficulty(ctx, difficulty)
	if err != nil {
		s.mu.Lock()
		s.waitingByDifficulty[difficulty] = append([]string{opponentID}, s.waitingByDifficulty[difficulty]...)
		s.mu.Unlock()
		return nil, false, err
	}

	match, err := s.matchService.CreateMatch(ctx, opponentID, userID, problem.ID, defaultMatchDurationSeconds)
	if err != nil {
		s.mu.Lock()
		s.waitingByDifficulty[difficulty] = append([]string{opponentID}, s.waitingByDifficulty[difficulty]...)
		s.mu.Unlock()
		return nil, false, err
	}
	return match, true, nil
}

func (s *Service) LeaveQueue(userID string) bool {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for difficulty, queue := range s.waitingByDifficulty {
		for i, queuedUserID := range queue {
			if queuedUserID != userID {
				continue
			}
			updated := append(queue[:i], queue[i+1:]...)
			if len(updated) == 0 {
				delete(s.waitingByDifficulty, difficulty)
			} else {
				s.waitingByDifficulty[difficulty] = updated
			}
			return true
		}
	}
	return false
}

func (s *Service) isQueuedLocked(userID string) bool {
	for _, queue := range s.waitingByDifficulty {
		for _, queuedUserID := range queue {
			if queuedUserID == userID {
				return true
			}
		}
	}
	return false
}
