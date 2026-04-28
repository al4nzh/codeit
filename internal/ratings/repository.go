package ratings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetRating loads current rating for a user.
func (r *Repository) GetRating(ctx context.Context, userID string) (int, error) {
	row := r.db.QueryRow(ctx, `SELECT rating FROM users WHERE id = $1`, userID)
	var rating int
	if err := row.Scan(&rating); err != nil {
		return 0, err
	}
	return rating, nil
}

// ApplyEloForMatch loads both ratings inside a transaction, computes new Elo from
// scoreForPlayer1 (1 = player1 win, 0 = player1 loss, 0.5 = draw), updates both users,
// and stores per-match rating snapshots when matchID is non-empty.
// If snapshots for that match already exist, the call is a no-op (idempotent).
func (r *Repository) ApplyEloForMatch(ctx context.Context, matchID, player1ID, player2ID string, scoreForPlayer1, k float64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	matchID = strings.TrimSpace(matchID)
	if matchID != "" {
		var existing sql.NullInt64
		err = tx.QueryRow(ctx, `
			SELECT player1_rating_after FROM matches WHERE id = $1 FOR UPDATE
		`, matchID).Scan(&existing)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("match %s not found for rating snapshot", matchID)
			}
			return err
		}
		if existing.Valid {
			return tx.Commit(ctx)
		}
	}

	var r1, r2 int
	err = tx.QueryRow(ctx, `
		SELECT rating FROM users WHERE id = $1 FOR UPDATE
	`, player1ID).Scan(&r1)
	if err != nil {
		return fmt.Errorf("player1 rating: %w", err)
	}
	err = tx.QueryRow(ctx, `
		SELECT rating FROM users WHERE id = $1 FOR UPDATE
	`, player2ID).Scan(&r2)
	if err != nil {
		return fmt.Errorf("player2 rating: %w", err)
	}

	newR1, newR2 := NewRatingsFromOutcome(r1, r2, scoreForPlayer1, k)
	d1 := newR1 - r1
	d2 := newR2 - r2

	tag1, err := tx.Exec(ctx, `UPDATE users SET rating = $1 WHERE id = $2`, newR1, player1ID)
	if err != nil {
		return err
	}
	if tag1.RowsAffected() != 1 {
		return fmt.Errorf("rating update: user %s not updated", player1ID)
	}

	tag2, err := tx.Exec(ctx, `UPDATE users SET rating = $1 WHERE id = $2`, newR2, player2ID)
	if err != nil {
		return err
	}
	if tag2.RowsAffected() != 1 {
		return fmt.Errorf("rating update: user %s not updated", player2ID)
	}

	if matchID != "" {
		_, err = tx.Exec(ctx, `
			UPDATE matches SET
				player1_rating_after = $1,
				player2_rating_after = $2,
				player1_elo_delta = $3,
				player2_elo_delta = $4
			WHERE id = $5 AND player1_rating_after IS NULL
		`, newR1, newR2, d1, d2, matchID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
