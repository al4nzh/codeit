package friendbattles

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"codeit/internal/matches"
	"codeit/internal/problems"
	"codeit/internal/users"
)

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrInvalidDifficulty   = errors.New("invalid difficulty")
	ErrAlreadyInMatch      = errors.New("user already has an active match")
	ErrInviteNotFound      = errors.New("invite not found")
	ErrInviteExpired       = errors.New("invite expired")
	ErrInviteAlreadyUsed   = errors.New("invite already used")
	ErrCannotJoinOwnInvite = errors.New("cannot join your own invite")
)

type MatchService interface {
	CreateMatch(ctx context.Context, player1ID, player2ID string, problemID int64, durationSeconds int, skipElo bool) (*matches.Match, error)
	GetActiveMatchByUserID(ctx context.Context, userID string) (*matches.Match, error)
}

type ProblemService interface {
	GetRandomProblemByDifficulty(ctx context.Context, difficulty string) (*problems.ProblemResponse, error)
}

type UserLookup interface {
	GetPublicProfile(ctx context.Context, id string) (*users.User, error)
}

const (
	defaultFriendMatchDurationSeconds = 20 * 60
	minFriendMatchDurationSeconds     = 60
	maxFriendMatchDurationSeconds     = 2 * 60 * 60
	inviteTTL                         = 24 * time.Hour
	maxCodeGenAttempts                = 12
)

var allowedDifficulties = map[string]struct{}{
	"easy":   {},
	"medium": {},
	"hard":   {},
}

type Service struct {
	db             *pgxpool.Pool
	repo           *Repository
	matchService   MatchService
	problemService ProblemService
	users          UserLookup
}

func NewService(db *pgxpool.Pool, repo *Repository, matchService MatchService, problemService ProblemService, users UserLookup) *Service {
	return &Service{
		db:             db,
		repo:           repo,
		matchService:   matchService,
		problemService: problemService,
		users:          users,
	}
}

func randomInviteCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, 8)
	for i := range b {
		out[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(out), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func normalizeDifficulty(difficulty string) (string, error) {
	d := strings.TrimSpace(strings.ToLower(difficulty))
	if d == "" {
		d = "easy"
	}
	if _, ok := allowedDifficulties[d]; !ok {
		return "", ErrInvalidDifficulty
	}
	return d, nil
}

func clampDuration(seconds int) int {
	if seconds <= 0 {
		return defaultFriendMatchDurationSeconds
	}
	if seconds < minFriendMatchDurationSeconds {
		return minFriendMatchDurationSeconds
	}
	if seconds > maxFriendMatchDurationSeconds {
		return maxFriendMatchDurationSeconds
	}
	return seconds
}

// CreateInvite stores a pending invite. Caller should share code (or join URL) with the guest.
func (s *Service) CreateInvite(ctx context.Context, hostUserID, difficulty string, durationSeconds int, skipElo bool) (*Invite, error) {
	hostUserID = strings.TrimSpace(hostUserID)
	if hostUserID == "" {
		return nil, ErrInvalidInput
	}
	d, err := normalizeDifficulty(difficulty)
	if err != nil {
		return nil, err
	}
	durationSeconds = clampDuration(durationSeconds)

	if _, err := s.matchService.GetActiveMatchByUserID(ctx, hostUserID); err == nil {
		return nil, ErrAlreadyInMatch
	}
	if err != nil && !errors.Is(err, matches.ErrMatchNotFound) {
		return nil, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(inviteTTL)

	var lastErr error
	for i := 0; i < maxCodeGenAttempts; i++ {
		code, err := randomInviteCode()
		if err != nil {
			return nil, err
		}
		inv := &Invite{
			ID:              uuid.New().String(),
			Code:            code,
			HostUserID:      hostUserID,
			Status:          StatusPending,
			Difficulty:      d,
			DurationSeconds: durationSeconds,
			SkipElo:         skipElo,
			ExpiresAt:       expiresAt,
			CreatedAt:       now,
		}
		if err := s.repo.InsertInvite(ctx, inv); err != nil {
			lastErr = err
			if isUniqueViolation(err) {
				continue
			}
			return nil, err
		}
		return inv, nil
	}
	if lastErr == nil {
		lastErr = errors.New("failed to generate unique invite code")
	}
	return nil, lastErr
}

// GetInviteForLanding returns invite details for an unauthenticated landing page.
func (s *Service) GetInviteForLanding(ctx context.Context, code string) (*Invite, *users.User, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil, ErrInviteNotFound
	}
	inv, err := s.repo.GetInviteByCode(ctx, strings.ToUpper(code))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrInviteNotFound
		}
		return nil, nil, err
	}
	if inv.Status != StatusPending && inv.Status != StatusAccepted {
		return nil, nil, ErrInviteNotFound
	}
	if inv.Status == StatusPending && time.Now().UTC().After(inv.ExpiresAt) {
		return nil, nil, ErrInviteExpired
	}
	host, err := s.users.GetPublicProfile(ctx, inv.HostUserID)
	if err != nil {
		if errors.Is(err, users.ErrUserNotFound) {
			return nil, nil, ErrInviteNotFound
		}
		return nil, nil, err
	}
	return inv, host, nil
}

// JoinWithCode locks the invite row, creates a running match (host = player1), and marks the invite accepted.
func (s *Service) JoinWithCode(ctx context.Context, guestUserID, code string) (*matches.Match, error) {
	guestUserID = strings.TrimSpace(guestUserID)
	code = strings.TrimSpace(code)
	if guestUserID == "" || code == "" {
		return nil, ErrInvalidInput
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inv, err := s.repo.LockInviteByCode(ctx, tx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInviteNotFound
		}
		return nil, err
	}
	if time.Now().UTC().After(inv.ExpiresAt) {
		return nil, ErrInviteExpired
	}
	if inv.Status == StatusAccepted {
		return nil, ErrInviteAlreadyUsed
	}
	if inv.Status != StatusPending {
		return nil, ErrInviteNotFound
	}
	if inv.HostUserID == guestUserID {
		return nil, ErrCannotJoinOwnInvite
	}

	if _, err := s.matchService.GetActiveMatchByUserID(ctx, inv.HostUserID); err == nil {
		return nil, ErrAlreadyInMatch
	}
	if err != nil && !errors.Is(err, matches.ErrMatchNotFound) {
		return nil, err
	}
	if _, err := s.matchService.GetActiveMatchByUserID(ctx, guestUserID); err == nil {
		return nil, ErrAlreadyInMatch
	}
	if err != nil && !errors.Is(err, matches.ErrMatchNotFound) {
		return nil, err
	}

	problem, err := s.problemService.GetRandomProblemByDifficulty(ctx, inv.Difficulty)
	if err != nil {
		return nil, err
	}

	match, err := s.matchService.CreateMatch(ctx, inv.HostUserID, guestUserID, problem.ID, inv.DurationSeconds, inv.SkipElo)
	if err != nil {
		return nil, err
	}

	if err := s.repo.MarkInviteAccepted(ctx, tx, inv.ID, match.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return match, nil
}
