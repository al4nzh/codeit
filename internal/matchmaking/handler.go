package matchmaking

import (
	"net/http"
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

func (h *Handler) Matchmake(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Difficulty string `json:"difficulty"`
	}
	_ = c.ShouldBindJSON(&req)

	difficulty := strings.TrimSpace(req.Difficulty)
	if difficulty == "" {
		difficulty = "easy"
	}

	match, matched, err := h.service.EnqueueOrMatch(c.Request.Context(), userID, difficulty)
	if err != nil {
		switch err {
		case ErrInvalidInput, ErrInvalidDifficulty:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case ErrAlreadyInQueue, ErrAlreadyInMatch:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to run matchmaking"})
		}
		return
	}

	if !matched {
		c.JSON(http.StatusAccepted, gin.H{
			"status":     "queued",
			"difficulty": strings.ToLower(difficulty),
		})
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

func (h *Handler) LeaveMatchmaking(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	left := h.service.LeaveQueue(userID)
	c.JSON(http.StatusOK, gin.H{"left_queue": left})
}
