package users

import (
	"context"
	"errors"
	"net/url"
	"regexp"
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
	ErrInvalidGoogleToken = errors.New("invalid google token")
	ErrGoogleUnavailable  = errors.New("google auth is not configured")
	ErrUsernameTaken      = errors.New("username is already taken")
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

type UserService struct {
	repo         *UserRepository
	googleVerify GoogleVerifier
}

type LoginResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

type GoogleVerifier interface {
	VerifyIDToken(ctx context.Context, rawToken string) (*auth.GoogleIdentity, error)
}

func NewUserService(repo *UserRepository, googleVerify GoogleVerifier) *UserService {
	return &UserService{repo: repo, googleVerify: googleVerify}
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

func (s *UserService) LoginWithGoogle(ctx context.Context, idToken string) (*LoginResponse, error) {
	idToken = strings.TrimSpace(idToken)
	if idToken == "" {
		return nil, ErrInvalidInput
	}
	if s.googleVerify == nil {
		return nil, ErrGoogleUnavailable
	}

	identity, err := s.googleVerify.VerifyIDToken(ctx, idToken)
	if err != nil || identity == nil {
		return nil, ErrInvalidGoogleToken
	}
	email := strings.TrimSpace(strings.ToLower(identity.Email))
	if email == "" {
		return nil, ErrInvalidGoogleToken
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}

		username, err := s.uniqueUsernameFromIdentity(ctx, identity.Name, email)
		if err != nil {
			return nil, err
		}
		hashedPassword, err := auth.HashPassword(uuid.New().String())
		if err != nil {
			return nil, err
		}
		newUser := &User{
			ID:        uuid.New().String(),
			Username:  username,
			Email:     email,
			Password:  hashedPassword,
			AvatarURL: "",
			Rating:    1200,
			CreatedAt: time.Now(),
		}
		if err := s.repo.CreateUser(ctx, newUser); err != nil {
			return nil, err
		}
		user = newUser
	} else if looksLikeGoogleProfileAvatarURL(user.AvatarURL) {
		if err := s.repo.UpdateAvatar(ctx, user.ID, ""); err != nil {
			return nil, err
		}
		user.AvatarURL = ""
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

func looksLikeGoogleProfileAvatarURL(raw string) bool {
	u := strings.TrimSpace(strings.ToLower(raw))
	if u == "" {
		return false
	}
	// Uploaded avatars are stored as local paths; never strip those.
	if strings.HasPrefix(u, "/uploads/") {
		return false
	}
	return strings.Contains(u, "googleusercontent.com")
}

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func (s *UserService) uniqueUsernameFromIdentity(ctx context.Context, name, email string) (string, error) {
	base := strings.TrimSpace(name)
	if base == "" {
		base = strings.Split(email, "@")[0]
	}
	base = nonAlnum.ReplaceAllString(base, "_")
	base = strings.Trim(base, "_")
	if len(base) < 3 {
		base = "coder"
	}
	if len(base) > 20 {
		base = base[:20]
	}

	candidate := base
	for i := 0; i < 8; i++ {
		exists, err := s.repo.ExistsByUsername(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
		suffix := uuid.New().String()[:6]
		if len(base) > 13 {
			candidate = base[:13] + "_" + suffix
		} else {
			candidate = base + "_" + suffix
		}
	}
	return "", errors.New("failed to generate unique username")
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

// UpdateMyUsername changes the authenticated user's username (case-insensitive uniqueness).
func (s *UserService) UpdateMyUsername(ctx context.Context, userID, newUsername string) (*User, error) {
	userID = strings.TrimSpace(userID)
	newUsername = strings.TrimSpace(newUsername)
	if userID == "" || newUsername == "" {
		return nil, ErrInvalidInput
	}
	if !usernamePattern.MatchString(newUsername) {
		return nil, ErrInvalidInput
	}

	cur, err := s.repo.GetById(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if strings.EqualFold(cur.Username, newUsername) {
		cur.Password = ""
		if err := s.attachRatingMeta(ctx, cur); err != nil {
			return nil, err
		}
		return cur, nil
	}

	taken, err := s.repo.ExistsByUsernameExcludingUser(ctx, newUsername, userID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrUsernameTaken
	}

	if err := s.repo.UpdateUsername(ctx, userID, newUsername); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	user, err := s.repo.GetPublicProfile(ctx, userID)
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
