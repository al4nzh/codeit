package friendbattles

import (
	"context"
	"errors"
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

func scanInvite(row pgx.Row) (*Invite, error) {
	var inv Invite
	var matchID *string
	if err := row.Scan(
		&inv.ID,
		&inv.Code,
		&inv.HostUserID,
		&inv.Status,
		&inv.Difficulty,
		&inv.DurationSeconds,
		&inv.SkipElo,
		&matchID,
		&inv.ExpiresAt,
		&inv.CreatedAt,
	); err != nil {
		return nil, err
	}
	inv.MatchID = matchID
	return &inv, nil
}

func (r *Repository) InsertInvite(ctx context.Context, inv *Invite) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO match_invites (
			id, code, host_user_id, status, difficulty, duration_seconds, skip_elo, match_id, expires_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, inv.ID, inv.Code, inv.HostUserID, inv.Status, inv.Difficulty, inv.DurationSeconds, inv.SkipElo, inv.MatchID, inv.ExpiresAt, inv.CreatedAt)
	return err
}

func (r *Repository) GetInviteByCode(ctx context.Context, code string) (*Invite, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, pgx.ErrNoRows
	}
	row := r.db.QueryRow(ctx, `
		SELECT id, code, host_user_id, status, difficulty, duration_seconds, skip_elo, match_id, expires_at, created_at
		FROM match_invites
		WHERE code = $1
	`, code)
	return scanInvite(row)
}

func (r *Repository) LockInviteByCode(ctx context.Context, tx pgx.Tx, code string) (*Invite, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, pgx.ErrNoRows
	}
	row := tx.QueryRow(ctx, `
		SELECT id, code, host_user_id, status, difficulty, duration_seconds, skip_elo, match_id, expires_at, created_at
		FROM match_invites
		WHERE code = $1
		FOR UPDATE
	`, code)
	return scanInvite(row)
}

func (r *Repository) MarkInviteAccepted(ctx context.Context, tx pgx.Tx, inviteID, matchID string) error {
	cmd, err := tx.Exec(ctx, `
		UPDATE match_invites
		SET status = $2, match_id = $3
		WHERE id = $1 AND status = $4
	`, inviteID, StatusAccepted, matchID, StatusPending)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return errors.New("invite not pending")
	}
	return nil
}
