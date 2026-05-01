package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetLatestByMatchAndUser(ctx context.Context, matchID, userID string) (*AnalyzeLastSubmissionResult, error) {
	row := r.db.QueryRow(ctx, `
		SELECT ma.id, ma.match_id, ma.user_id, COALESCE(ma.submission_id, ''), ma.language, COALESCE(s.code, ''), ma.passed_count, ma.total_count,
		       summary, strengths, issues, suggestions, score, analyzed_at
		FROM match_analyses ma
		LEFT JOIN submissions s ON s.id = ma.submission_id
		WHERE ma.match_id = $1 AND ma.user_id = $2
		ORDER BY ma.analyzed_at DESC
		LIMIT 1
	`, matchID, userID)
	out, err := scanAnalysisRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return out, nil
}

func (r *Repository) Save(ctx context.Context, in *AnalyzeLastSubmissionResult) (*AnalyzeLastSubmissionResult, error) {
	if in == nil {
		return nil, errors.New("analysis payload is nil")
	}
	if in.ID == "" {
		in.ID = uuid.New().String()
	}
	if in.AnalyzedAt.IsZero() {
		in.AnalyzedAt = time.Now()
	}
	strengths, _ := json.Marshal(in.Strengths)
	issues, _ := json.Marshal(in.Issues)
	suggestions, _ := json.Marshal(in.Suggestions)

	_, err := r.db.Exec(ctx, `
		INSERT INTO match_analyses (
			id, match_id, user_id, submission_id, language, passed_count, total_count,
			summary, strengths, issues, suggestions, score, analyzed_at
		) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11::jsonb,$12,$13)
	`, in.ID, in.MatchID, in.UserID, in.SubmissionID, in.Language, in.PassedCount, in.TotalCount,
		in.Summary, string(strengths), string(issues), string(suggestions), in.Score, in.AnalyzedAt)
	if err != nil {
		return nil, err
	}
	return in, nil
}

func (r *Repository) ListByUser(ctx context.Context, userID string, limit, offset int) (*HistoryPage, error) {
	row := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM match_analyses WHERE user_id = $1`, userID)
	var total int64
	if err := row.Scan(&total); err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT ma.id, ma.match_id, ma.user_id, COALESCE(ma.submission_id, ''), ma.language, COALESCE(s.code, ''), ma.passed_count, ma.total_count,
		       summary, strengths, issues, suggestions, score, analyzed_at
		FROM match_analyses ma
		LEFT JOIN submissions s ON s.id = ma.submission_id
		WHERE ma.user_id = $1
		ORDER BY ma.analyzed_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AnalyzeLastSubmissionResult, 0)
	for rows.Next() {
		item, err := scanAnalysisRows(rows)
		if err != nil {
			return nil, err
		}
		item.Cached = true
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &HistoryPage{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanAnalysisRow(row rowScanner) (*AnalyzeLastSubmissionResult, error) {
	var strengths, issues, suggestions []byte
	out := &AnalyzeLastSubmissionResult{}
	if err := row.Scan(
		&out.ID, &out.MatchID, &out.UserID, &out.SubmissionID, &out.Language, &out.Code, &out.PassedCount, &out.TotalCount,
		&out.Summary, &strengths, &issues, &suggestions, &out.Score, &out.AnalyzedAt,
	); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(strengths, &out.Strengths)
	_ = json.Unmarshal(issues, &out.Issues)
	_ = json.Unmarshal(suggestions, &out.Suggestions)
	return out, nil
}

func scanAnalysisRows(rows pgx.Rows) (*AnalyzeLastSubmissionResult, error) {
	return scanAnalysisRow(rows)
}
