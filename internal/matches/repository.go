package matches

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

const matchSelectColumns = `id, player1_id, player2_id, problem_id, status, duration_seconds, started_at, ended_at, winner_id, created_at, player1_rating_after, player2_rating_after, player1_elo_delta, player2_elo_delta`

func assignRatingSnapshot(m *Match, p1a, p2a, p1d, p2d sql.NullInt64) {
	if p1a.Valid {
		v := int(p1a.Int64)
		m.Player1RatingAfter = &v
	}
	if p2a.Valid {
		v := int(p2a.Int64)
		m.Player2RatingAfter = &v
	}
	if p1d.Valid {
		v := int(p1d.Int64)
		m.Player1EloDelta = &v
	}
	if p2d.Valid {
		v := int(p2d.Int64)
		m.Player2EloDelta = &v
	}
}

type MatchRepository struct {
	db *pgxpool.Pool
}

func NewMatchRepository(db *pgxpool.Pool) *MatchRepository {
	return &MatchRepository{db: db}
}

func (r *MatchRepository) CreateMatch(ctx context.Context, match *Match) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO matches (
			id, player1_id, player2_id, problem_id, status, duration_seconds,
			started_at, ended_at, winner_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, match.ID, match.Player1ID, match.Player2ID, match.ProblemID, match.Status, match.DurationSeconds,
		match.StartedAt, match.EndedAt, match.WinnerID, match.CreatedAt)
	return err
}

func (r *MatchRepository) GetByID(ctx context.Context, id string) (*Match, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+matchSelectColumns+`
		FROM matches
		WHERE id = $1
	`, id)

	var match Match
	var p1a, p2a, p1d, p2d sql.NullInt64
	if err := row.Scan(
		&match.ID,
		&match.Player1ID,
		&match.Player2ID,
		&match.ProblemID,
		&match.Status,
		&match.DurationSeconds,
		&match.StartedAt,
		&match.EndedAt,
		&match.WinnerID,
		&match.CreatedAt,
		&p1a, &p2a, &p1d, &p2d,
	); err != nil {
		return nil, err
	}
	assignRatingSnapshot(&match, p1a, p2a, p1d, p2d)

	return &match, nil
}

func (r *MatchRepository) GetActiveMatchByUserID(ctx context.Context, userID string) (*Match, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+matchSelectColumns+`
		FROM matches
		WHERE (player1_id = $1 OR player2_id = $1) AND status IN ('waiting', 'running')
		ORDER BY created_at DESC
		LIMIT 1
	`, userID)

	var match Match
	var p1a, p2a, p1d, p2d sql.NullInt64
	if err := row.Scan(
		&match.ID,
		&match.Player1ID,
		&match.Player2ID,
		&match.ProblemID,
		&match.Status,
		&match.DurationSeconds,
		&match.StartedAt,
		&match.EndedAt,
		&match.WinnerID,
		&match.CreatedAt,
		&p1a, &p2a, &p1d, &p2d,
	); err != nil {
		return nil, err
	}
	assignRatingSnapshot(&match, p1a, p2a, p1d, p2d)

	return &match, nil
}

func (r *MatchRepository) CountFinishedMatchesForUser(ctx context.Context, userID string) (int64, error) {
	row := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM matches
		WHERE (player1_id = $1 OR player2_id = $1) AND status = 'finished'
	`, userID)
	var n int64
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *MatchRepository) ListFinishedMatchesForUser(ctx context.Context, userID string, limit, offset int) ([]Match, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+matchSelectColumns+`
		FROM matches
		WHERE (player1_id = $1 OR player2_id = $1) AND status = 'finished'
		ORDER BY ended_at DESC NULLS LAST, created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Match
	for rows.Next() {
		var m Match
		var p1a, p2a, p1d, p2d sql.NullInt64
		if err := rows.Scan(
			&m.ID,
			&m.Player1ID,
			&m.Player2ID,
			&m.ProblemID,
			&m.Status,
			&m.DurationSeconds,
			&m.StartedAt,
			&m.EndedAt,
			&m.WinnerID,
			&m.CreatedAt,
			&p1a, &p2a, &p1d, &p2d,
		); err != nil {
			return nil, err
		}
		assignRatingSnapshot(&m, p1a, p2a, p1d, p2d)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MatchRepository) GetUserMatchStats(ctx context.Context, userID string) (UserMatchStats, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*)::bigint AS played,
			COUNT(*) FILTER (WHERE winner_id = $1)::bigint AS wins,
			COUNT(*) FILTER (WHERE winner_id IS NOT NULL AND winner_id <> $1)::bigint AS losses,
			COUNT(*) FILTER (WHERE winner_id IS NULL)::bigint AS draws
		FROM matches
		WHERE (player1_id = $1 OR player2_id = $1) AND status = 'finished'
	`, userID)
	var s UserMatchStats
	if err := row.Scan(&s.MatchesPlayed, &s.Wins, &s.Losses, &s.Draws); err != nil {
		return UserMatchStats{}, err
	}
	return s, nil
}

func (r *MatchRepository) StartMatch(ctx context.Context, id string, startedAt time.Time) error {
	cmdTag, err := r.db.Exec(ctx, `
		UPDATE matches
		SET status = 'running', started_at = $1
		WHERE id = $2 AND status = 'waiting'
	`, startedAt, id)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("match not in waiting state")
	}
	return err
}

func (r *MatchRepository) FinishMatch(ctx context.Context, id string, winnerID *string, endedAt time.Time) error {
	cmdTag, err := r.db.Exec(ctx, `
		UPDATE matches
		SET status = 'finished', winner_id = $1, ended_at = $2
		WHERE id = $3 AND status = 'running'
	`, winnerID, endedAt, id)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("match not in running state")
	}
	return err
}
