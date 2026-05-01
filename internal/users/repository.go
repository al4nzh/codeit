package users

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) CreateUser(ctx context.Context, user *User) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users (id, username, email, password, avatar_url, rating, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, user.ID, user.Username, user.Email, user.Password, user.AvatarURL, user.Rating, user.CreatedAt)
	return err
}

func (r *UserRepository) GetById(ctx context.Context, id string) (*User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, username, email, password, COALESCE(avatar_url, ''), rating, created_at
		FROM users
		WHERE id = $1
	`, id)
	var user User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.AvatarURL, &user.Rating, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil

}
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, username, email, password, COALESCE(avatar_url, ''), rating, created_at
		FROM users
		WHERE email = $1
	`, email)
	var user User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.Password, &user.AvatarURL, &user.Rating, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	row := r.db.QueryRow(ctx, `
		SELECT 1
		FROM users
		WHERE LOWER(username) = LOWER($1)
		LIMIT 1
	`, username)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *UserRepository) ExistsByUsernameExcludingUser(ctx context.Context, username, excludeUserID string) (bool, error) {
	row := r.db.QueryRow(ctx, `
		SELECT 1
		FROM users
		WHERE LOWER(username) = LOWER($1) AND id <> $2
		LIMIT 1
	`, username, excludeUserID)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *UserRepository) UpdateUsername(ctx context.Context, userID, username string) error {
	cmdTag, err := r.db.Exec(ctx, `
		UPDATE users
		SET username = $1
		WHERE id = $2
	`, username, userID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *UserRepository) UpdateRating(ctx context.Context, id string, rating int) error {
	cmdTag, err := r.db.Exec(ctx, `
		UPDATE users
		SET rating = $1
		WHERE id = $2
	`, rating, id)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (r *UserRepository) GetPublicProfile(ctx context.Context, id string) (*User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, username, COALESCE(avatar_url, ''), rating, created_at
		FROM users
		WHERE id = $1
	`, id)
	var user User
	err := row.Scan(&user.ID, &user.Username, &user.AvatarURL, &user.Rating, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetPublicProfileByUsername(ctx context.Context, username string) (*User, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, username, COALESCE(avatar_url, ''), rating, created_at
		FROM users
		WHERE LOWER(username) = LOWER($1)
	`, username)
	var user User
	err := row.Scan(&user.ID, &user.Username, &user.AvatarURL, &user.Rating, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetWorldRank returns 1-based rank: 1 + number of users with strictly higher rating.
func (r *UserRepository) GetWorldRank(ctx context.Context, userID string) (int, error) {
	row := r.db.QueryRow(ctx, `
		SELECT 1 + COALESCE((
			SELECT COUNT(*)::int
			FROM users u
			WHERE u.rating > (SELECT rating FROM users WHERE id = $1)
		), 0)
	`, userID)
	var rank int
	if err := row.Scan(&rank); err != nil {
		return 0, err
	}
	return rank, nil
}

func (r *UserRepository) ListLeaderboard(ctx context.Context, limit, offset int) ([]LeaderboardEntry, error) {
	rows, err := r.db.Query(ctx, `
		SELECT world_rank, id, username, COALESCE(avatar_url, ''), rating
		FROM (
			SELECT
				id,
				username,
				avatar_url,
				rating,
				RANK() OVER (ORDER BY rating DESC, created_at ASC) AS world_rank
			FROM users
		) ranked
		ORDER BY world_rank ASC, username ASC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LeaderboardEntry, 0)
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.WorldRank, &e.UserID, &e.Username, &e.AvatarURL, &e.Rating); err != nil {
			return nil, err
		}
		e.Title = TitleForRating(e.Rating)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *UserRepository) UpdateAvatar(ctx context.Context, id, avatarURL string) error {
	cmdTag, err := r.db.Exec(ctx, `
		UPDATE users
		SET avatar_url = $1
		WHERE id = $2
	`, avatarURL, id)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}
