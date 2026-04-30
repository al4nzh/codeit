package friendbattles

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"codeit/internal/auth"
	"codeit/internal/ws"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
	hub     *ws.Hub
}

func NewHandler(service *Service, hub *ws.Hub) *Handler {
	return &Handler{service: service, hub: hub}
}

func joinPath(code string) string {
	return "/friend-battle/" + strings.TrimSpace(code)
}

func (h *Handler) CreateInvite(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Difficulty        string `json:"difficulty"`
		DurationSeconds   int    `json:"duration_seconds"`
		SkipElo           *bool  `json:"skip_elo"`
	}
	_ = c.ShouldBindJSON(&req)

	skipElo := true
	if req.SkipElo != nil {
		skipElo = *req.SkipElo
	}

	inv, err := h.service.CreateInvite(c.Request.Context(), userID, req.Difficulty, req.DurationSeconds, skipElo)
	if err != nil {
		switch err {
		case ErrInvalidInput, ErrInvalidDifficulty:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case ErrAlreadyInMatch:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create invite"})
		}
		return
	}

	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_APP_URL")), "/")
	out := gin.H{
		"code":               inv.Code,
		"join_path":          joinPath(inv.Code),
		"expires_at":         inv.ExpiresAt,
		"difficulty":         inv.Difficulty,
		"duration_seconds":   inv.DurationSeconds,
		"skip_elo":           inv.SkipElo,
		"host_user_id":       inv.HostUserID,
		"status":             inv.Status,
	}
	if base != "" {
		out["join_url"] = base + joinPath(inv.Code)
	}
	c.JSON(http.StatusCreated, out)
}

func (h *Handler) GetInvite(c *gin.Context) {
	code := c.Param("code")
	inv, host, err := h.service.GetInviteForLanding(c.Request.Context(), code)
	if err != nil {
		switch err {
		case ErrInviteNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrInviteExpired:
			c.JSON(http.StatusGone, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load invite"})
		}
		return
	}

	hostOut := gin.H{
		"id":            host.ID,
		"username":      host.Username,
		"avatar_url":    host.AvatarURL,
		"rating":        host.Rating,
		"world_rank":    host.WorldRank,
		"rating_title":  host.RatingTitle,
	}
	c.JSON(http.StatusOK, gin.H{
		"code":             inv.Code,
		"status":           inv.Status,
		"difficulty":       inv.Difficulty,
		"duration_seconds": inv.DurationSeconds,
		"skip_elo":         inv.SkipElo,
		"expires_at":       inv.ExpiresAt,
		"match_id":         inv.MatchID,
		"host":             hostOut,
		"join_path":        joinPath(inv.Code),
	})
}

func (h *Handler) JoinInvite(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	code := c.Param("code")
	match, err := h.service.JoinWithCode(c.Request.Context(), userID, code)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInviteNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInviteExpired):
			c.JSON(http.StatusGone, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInviteAlreadyUsed):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrCannotJoinOwnInvite):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case errors.Is(err, ErrAlreadyInMatch):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to join invite"})
		}
		return
	}

	event := ws.Event{
		Type:    "match_found",
		Payload: match,
	}
	_ = h.hub.BroadcastToUser(match.Player1ID, event)
	_ = h.hub.BroadcastToUser(match.Player2ID, event)

	c.JSON(http.StatusOK, gin.H{
		"status": "matched",
		"match":  match,
	})
}
