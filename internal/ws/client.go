package ws

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 * 8
)

type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	userID  string
	matchID string
	matchSvc MatchService
}

type incomingEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type incomingChatPayload struct {
	Text string `json:"text"`
}

func NewClient(hub *Hub, conn *websocket.Conn, userID, matchID string, matchSvc MatchService) *Client {
	return &Client{
		hub:     hub,
		conn:    conn,
		send:    make(chan []byte, 16),
		userID:  userID,
		matchID: matchID,
		matchSvc: matchSvc,
	}
}

func (c *Client) canChatInMatch(ctx context.Context) bool {
	if c.matchSvc == nil || c.matchID == "" {
		return false
	}
	match, err := c.matchSvc.GetByID(ctx, c.matchID)
	if err != nil {
		return false
	}
	return match.Player1ID == c.userID || match.Player2ID == c.userID
}

func (c *Client) handleIncoming(raw []byte) {
	var event incomingEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return
	}
	if event.Type != "chat_message" {
		return
	}
	if c.matchID == "" {
		return
	}
	if !c.canChatInMatch(context.Background()) {
		return
	}

	var payload incomingChatPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return
	}

	text := strings.TrimSpace(payload.Text)
	if text == "" || utf8.RuneCountInString(text) > 200 {
		return
	}

	_ = c.hub.BroadcastToMatch(c.matchID, Event{
		Type: "chat_message",
		Payload: map[string]any{
			"user_id": c.userID,
			"text":    text,
			"sent_at": time.Now().UTC(),
		},
	})
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		c.handleIncoming(message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
