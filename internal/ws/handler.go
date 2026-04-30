package ws

import (
	"context"
	"net/http"
	"strings"

	"codeit/internal/auth"
	"codeit/internal/matches"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type Handler struct {
	hub      *Hub
	matchSvc MatchService
	upgrader websocket.Upgrader
}

type MatchService interface {
	GetByID(ctx context.Context, id string) (*matches.Match, error)
}

func NewHandler(hub *Hub, matchSvc MatchService) *Handler {
	return &Handler{
		hub:      hub,
		matchSvc: matchSvc,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				// MVP: allow all origins until frontend origin is fixed.
				return true
			},
		},
	}
}

func (h *Handler) HandleWebSocket(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	matchID := strings.TrimSpace(c.Query("match_id"))

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := NewClient(h.hub, conn, userID, matchID, h.matchSvc)
	h.hub.Register(client)

	go client.writePump()
	go client.readPump()
}
