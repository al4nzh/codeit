package matches

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
)

const (
	defaultHistoryLimit = 20
	maxHistoryLimit     = 100
)

var (
	ErrInvalidMatchID = errors.New("invalid match id")
	ErrInvalidUserID  = errors.New("invalid user id")
	ErrInvalidInput   = errors.New("invalid input")
	ErrMatchNotFound  = errors.New("match not found")
	ErrInvalidWinner  = errors.New("winner must be one of the two match players")
	ErrInvalidVictory = errors.New("invalid victory type")
	ErrInvalidState   = errors.New("invalid match state transition")
)

type MatchService struct {
	repo *MatchRepository
}

func NewMatchService(repo *MatchRepository) *MatchService {
	return &MatchService{repo: repo}
}

func (s *MatchService) CreateMatch(ctx context.Context, player1ID, player2ID string, problemID int64, durationSeconds int) (*Match, error) {
	player1ID = strings.TrimSpace(player1ID)
	player2ID = strings.TrimSpace(player2ID)
	if player1ID == "" || player2ID == "" || problemID <= 0 || player1ID == player2ID || durationSeconds <= 0 {
		return nil, ErrInvalidInput
	}

	startedAt := time.Now()
	match := &Match{
		ID:              uuid.New().String(),
		Player1ID:       player1ID,
		Player2ID:       player2ID,
		ProblemID:       problemID,
		Status:          StatusRunning,
		DurationSeconds: durationSeconds,
		StartedAt:       &startedAt,
		CreatedAt:       time.Now(),
	}

	if err := s.repo.CreateMatch(ctx, match); err != nil {
		return nil, err
	}
	match.Result = ResolveMatchResult(match)

	return match, nil
}

func (s *MatchService) GetByID(ctx context.Context, id string) (*Match, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrInvalidMatchID
	}

	match, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMatchNotFound
		}
		return nil, err
	}
	match.Result = ResolveMatchResult(match)
	match.VictoryType = ResolveVictoryType(match)
	return match, nil
}

func (s *MatchService) ListFinishedMatchHistory(ctx context.Context, userID string, limit, offset int) (*MatchHistoryPage, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidUserID
	}
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}
	if offset < 0 {
		offset = 0
	}

	total, err := s.repo.CountFinishedMatchesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	raw, err := s.repo.ListFinishedMatchesForUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	entries := make([]MatchHistoryEntry, 0, len(raw))
	for i := range raw {
		m := &raw[i]
		m.Result = ResolveMatchResult(m)
		m.VictoryType = ResolveVictoryType(m)
		entries = append(entries, MatchHistoryEntry{
			Match:         *m,
			OpponentID:    OpponentID(m, userID),
			MyResult:      MyResultForUser(m, userID),
			MyRatingAfter: MyRatingAfterForUser(m, userID),
			MyEloDelta:    MyEloDeltaForUser(m, userID),
		})
	}

	return &MatchHistoryPage{
		Matches: entries,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

func (s *MatchService) GetUserMatchStats(ctx context.Context, userID string) (*UserMatchStats, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidUserID
	}
	stats, err := s.repo.GetUserMatchStats(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

func (s *MatchService) GetActiveMatchByUserID(ctx context.Context, userID string) (*Match, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	match, err := s.repo.GetActiveMatchByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMatchNotFound
		}
		return nil, err
	}
	match.Result = ResolveMatchResult(match)
	match.VictoryType = ResolveVictoryType(match)
	return match, nil
}

func (s *MatchService) StartMatch(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return ErrInvalidMatchID
	}
	match, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMatchNotFound
		}
		return err
	}
	if match.Status != StatusWaiting {
		return ErrInvalidState
	}
	return s.repo.StartMatch(ctx, id, time.Now())
}

func (s *MatchService) FinishMatch(ctx context.Context, id string, winnerID string) error {
	victoryType := VictoryTypeDecision
	if strings.TrimSpace(winnerID) == "" {
		victoryType = VictoryTypeDraw
	}
	return s.FinishMatchWithVictoryType(ctx, id, winnerID, victoryType)
}

func (s *MatchService) FinishMatchWithVictoryType(ctx context.Context, id string, winnerID, victoryType string) error {
	id = strings.TrimSpace(id)
	winnerID = strings.TrimSpace(winnerID)
	victoryType = strings.TrimSpace(strings.ToLower(victoryType))
	if id == "" {
		return ErrInvalidMatchID
	}
	switch victoryType {
	case VictoryTypeKO, VictoryTypeDecision, VictoryTypeDraw:
	default:
		return ErrInvalidVictory
	}
	match, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMatchNotFound
		}
		return err
	}
	if match.Status != StatusRunning {
		return ErrInvalidState
	}

	var winnerIDPtr *string
	if winnerID != "" {
		if winnerID != match.Player1ID && winnerID != match.Player2ID {
			return ErrInvalidWinner
		}
		if victoryType == VictoryTypeDraw {
			return ErrInvalidVictory
		}
		winnerIDPtr = &winnerID
	} else if victoryType != VictoryTypeDraw {
		return ErrInvalidVictory
	}

	return s.repo.FinishMatch(ctx, id, winnerIDPtr, time.Now(), victoryType)
}
