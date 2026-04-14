package ws

import (
	"sync"

	"github.com/google/uuid"
)

// Hub manages WebSocket rooms, one room per game.
type Hub struct {
	mu    sync.RWMutex
	rooms map[int32]*room
}

type room struct {
	mu      sync.Mutex
	clients map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[int32]*room)}
}

func (h *Hub) Join(gameID int32, c *Client) {
	h.mu.Lock()
	r, ok := h.rooms[gameID]
	if !ok {
		r = &room{clients: make(map[*Client]struct{})}
		h.rooms[gameID] = r
	}
	h.mu.Unlock()

	r.mu.Lock()
	r.clients[c] = struct{}{}
	r.mu.Unlock()
}

func (h *Hub) Leave(gameID int32, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r, ok := h.rooms[gameID]
	if !ok {
		return
	}

	r.mu.Lock()
	delete(r.clients, c)
	empty := len(r.clients) == 0
	r.mu.Unlock()

	if empty {
		delete(h.rooms, gameID)
	}
}

func (h *Hub) Broadcast(gameID int32, msg []byte) {
	h.mu.RLock()
	r, ok := h.rooms[gameID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for c := range r.clients {
		c.Send(msg)
	}
}

func (h *Hub) SendToUser(gameID int32, userID uuid.UUID, msg []byte) {
	h.mu.RLock()
	r, ok := h.rooms[gameID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for c := range r.clients {
		if c.UserID == userID {
			c.Send(msg)
			return
		}
	}
}
