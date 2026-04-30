package ratings

import (
	"context"
	"errors"

	"codeit/internal/matches"
)

var (
	ErrMatchNotFinished = errors.New("match is not finished")
	ErrInvalidMatch     = errors.New("match has invalid players for rating update")
)

type Service struct {
	repo *Repository
	k    float64
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo, k: defaultK}
}

// ApplyFinishedMatch updates both players' Elo after a finished ranked match.
// Player order follows the match: player1_id vs player2_id; outcome uses matches.Result.
func (s *Service) ApplyFinishedMatch(ctx context.Context, match *matches.Match) error {
	if match == nil {
		return ErrInvalidMatch
	}
	if match.Status != matches.StatusFinished {
		return ErrMatchNotFinished
	}
	if match.SkipElo {
		return nil
	}
	if match.Player1ID == "" || match.Player2ID == "" || match.Player1ID == match.Player2ID {
		return ErrInvalidMatch
	}

	result := matches.ResolveMatchResult(match)
	var scoreForPlayer1 float64
	switch result {
	case matches.ResultPlayer1:
		scoreForPlayer1 = 1.0
	case matches.ResultPlayer2:
		scoreForPlayer1 = 0.0
	case matches.ResultDraw:
		scoreForPlayer1 = 0.5
	default:
		return ErrMatchNotFinished
	}

	return s.repo.ApplyEloForMatch(ctx, match.ID, match.Player1ID, match.Player2ID, scoreForPlayer1, s.k)
}
