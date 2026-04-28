package ws

import (
	"encoding/json"
	"sync"
)

type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]struct{}
	userClients map[string]map[*Client]struct{}
	matchRooms map[string]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients:     make(map[*Client]struct{}),
		userClients: make(map[string]map[*Client]struct{}),
		matchRooms:  make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = struct{}{}

	if _, ok := h.userClients[client.userID]; !ok {
		h.userClients[client.userID] = make(map[*Client]struct{})
	}
	h.userClients[client.userID][client] = struct{}{}

	if client.matchID != "" {
		if _, ok := h.matchRooms[client.matchID]; !ok {
			h.matchRooms[client.matchID] = make(map[*Client]struct{})
		}
		h.matchRooms[client.matchID][client] = struct{}{}
	}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, client)

	if byUser, ok := h.userClients[client.userID]; ok {
		delete(byUser, client)
		if len(byUser) == 0 {
			delete(h.userClients, client.userID)
		}
	}

	if client.matchID != "" {
		if room, ok := h.matchRooms[client.matchID]; ok {
			delete(room, client)
			if len(room) == 0 {
				delete(h.matchRooms, client.matchID)
			}
		}
	}
}

func (h *Hub) BroadcastToUser(userID string, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	h.mu.RLock()
	targets := h.userClients[userID]
	h.mu.RUnlock()

	for client := range targets {
		select {
		case client.send <- payload:
		default:
		}
	}

	return nil
}

func (h *Hub) BroadcastToMatch(matchID string, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	h.mu.RLock()
	targets := h.matchRooms[matchID]
	h.mu.RUnlock()

	for client := range targets {
		select {
		case client.send <- payload:
		default:
		}
	}

	return nil
}
