package submissions

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type SubmissionRepository struct {
	db *pgxpool.Pool
}

func NewSubmissionRepository(db *pgxpool.Pool) *SubmissionRepository {
	return &SubmissionRepository{db: db}
}

func (r *SubmissionRepository) CreateSubmission(ctx context.Context, submission *Submission) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO submissions (
			id, match_id, user_id, language, code, passed_count, total_count, status, submitted_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, submission.ID, submission.MatchID, submission.UserID, submission.Language, submission.Code,
		submission.PassedCount, submission.TotalCount, submission.Status, submission.SubmittedAt)
	return err
}

func (r *SubmissionRepository) GetBestPassedCountByMatchAndUser(ctx context.Context, matchID, userID string) (int, error) {
	row := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(passed_count), 0)
		FROM submissions
		WHERE match_id = $1 AND user_id = $2
	`, matchID, userID)

	var best int
	if err := row.Scan(&best); err != nil {
		return 0, err
	}
	return best, nil
}

func (r *SubmissionRepository) GetLastSubmissionByMatchAndUser(ctx context.Context, matchID, userID string) (*Submission, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, match_id, user_id, language, code, passed_count, total_count, status, submitted_at
		FROM submissions
		WHERE match_id = $1 AND user_id = $2
		ORDER BY submitted_at DESC
		LIMIT 1
	`, matchID, userID)

	var s Submission
	if err := row.Scan(
		&s.ID, &s.MatchID, &s.UserID, &s.Language, &s.Code, &s.PassedCount, &s.TotalCount, &s.Status, &s.SubmittedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}
