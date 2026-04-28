package users

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"codeit/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
)

type UserService struct {
	repo *UserRepository
}

type LoginResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

func NewUserService(repo *UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Register(ctx context.Context, username, email, password string) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(strings.ToLower(email))

	if username == "" || email == "" || len(password) < 6 {
		return nil, ErrInvalidInput
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:        uuid.New().String(),
		Username:  username,
		Email:     email,
		Password:  hashedPassword,
		Rating:    1200,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	user.Password = ""
	if err := s.attachRatingMeta(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := auth.CheckPassword(user.Password, password); err != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		return nil, err
	}

	user.Password = ""
	if err := s.attachRatingMeta(ctx, user); err != nil {
		return nil, err
	}
	return &LoginResponse{Token: token, User: user}, nil
}

func (s *UserService) GetPublicProfile(ctx context.Context, id string) (*User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.repo.GetPublicProfile(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if err := s.attachRatingMeta(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetPublicProfileByUsername(ctx context.Context, username string) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.repo.GetPublicProfileByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if err := s.attachRatingMeta(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetLeaderboard(ctx context.Context, limit, offset int) ([]LeaderboardEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListLeaderboard(ctx, limit, offset)
}

func (s *UserService) attachRatingMeta(ctx context.Context, u *User) error {
	if u == nil {
		return nil
	}
	rank, err := s.repo.GetWorldRank(ctx, u.ID)
	if err != nil {
		return err
	}
	u.WorldRank = rank
	u.RatingTitle = TitleForRating(u.Rating)
	return nil
}

func (s *UserService) UpdateRating(ctx context.Context, id string, rating int) error {
	id = strings.TrimSpace(id)
	if id == "" || rating < 0 {
		return ErrInvalidInput
	}

	return s.repo.UpdateRating(ctx, id, rating)
}

func (s *UserService) UpdateAvatar(ctx context.Context, id, avatarURL string) error {
	id = strings.TrimSpace(id)
	avatarURL = strings.TrimSpace(avatarURL)
	if id == "" {
		return ErrInvalidInput
	}
	if avatarURL != "" {
		// Allow either absolute URLs (http/https) or local static paths (/uploads/...).
		if strings.HasPrefix(avatarURL, "/") {
			// ok
		} else {
			parsed, err := url.ParseRequestURI(avatarURL)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				return ErrInvalidInput
			}
		}
	}
	return s.repo.UpdateAvatar(ctx, id, avatarURL)
}
