package submissions

import (
	"net/http"

	"codeit/internal/auth"
	"codeit/internal/matches"
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

func (h *Handler) Submit(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	var req struct {
		Language string `json:"language"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	res, err := h.service.Submit(c.Request.Context(), matchID, userID, req.Language, req.Code)
	if err != nil {
		switch err {
		case ErrInvalidInput, ErrUnsupportedLang:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case ErrUnauthorizedForMatch:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case ErrMatchNotRunning, ErrMatchExpired:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case matches.ErrMatchNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit solution"})
		}
		return
	}

	_ = h.hub.BroadcastToMatch(matchID, ws.Event{
		Type: "submission_received",
		Payload: gin.H{
			"user_id":      userID,
			"passed_count": res.Submission.PassedCount,
			"total_count":  res.Submission.TotalCount,
			"submitted_at": res.Submission.SubmittedAt,
		},
	})

	if res.MatchFinished {
		match, err := h.service.matchService.GetByID(c.Request.Context(), matchID)
		if err == nil {
			_ = h.hub.BroadcastToMatch(matchID, ws.Event{
				Type:    "match_ended",
				Payload: match,
			})
		}
	}

	c.JSON(http.StatusCreated, res)
}

// ResolveMatch finishes a match after the timer has elapsed (no Judge0 run). Same outcome as
// submitting after the deadline: winner by best passed_count, or draw.
func (h *Handler) ResolveMatch(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	matchID := c.Param("id")
	out, err := h.service.ResolveExpiredMatch(c.Request.Context(), matchID, userID)
	if err != nil {
		switch err {
		case ErrInvalidInput:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case ErrUnauthorizedForMatch:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case ErrMatchNotRunning:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case ErrMatchNotExpiredYet:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case matches.ErrMatchNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve match"})
		}
		return
	}

	if out.Resolved {
		_ = h.hub.BroadcastToMatch(matchID, ws.Event{
			Type:    "match_ended",
			Payload: out.Match,
		})
	}

	c.JSON(http.StatusOK, out)
}
