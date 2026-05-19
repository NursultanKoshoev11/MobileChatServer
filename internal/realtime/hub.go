package realtime

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

type Event struct {
	Type    string `json:"type"`
	GroupID string `json:"group_id,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

type Hub struct {
	logger *log.Logger
	mu     sync.RWMutex
	groups map[string]map[*Client]bool
	users  map[string]map[*Client]bool
}

func NewHub(logger *log.Logger) *Hub {
	return &Hub{logger: logger, groups: make(map[string]map[*Client]bool), users: make(map[string]map[*Client]bool)}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if client.GroupID != "" {
		if h.groups[client.GroupID] == nil {
			h.groups[client.GroupID] = make(map[*Client]bool)
		}
		h.groups[client.GroupID][client] = true
	}
	if h.users[client.UserID] == nil {
		h.users[client.UserID] = make(map[*Client]bool)
	}
	h.users[client.UserID][client] = true
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	closed := false
	if client.GroupID != "" {
		if clients := h.groups[client.GroupID]; clients != nil {
			if clients[client] {
				delete(clients, client)
				close(client.Send)
				closed = true
			}
			if len(clients) == 0 {
				delete(h.groups, client.GroupID)
			}
		}
	}
	if clients := h.users[client.UserID]; clients != nil {
		if clients[client] {
			delete(clients, client)
			if !closed {
				close(client.Send)
			}
		}
		if len(clients) == 0 {
			delete(h.users, client.UserID)
		}
	}
}

func (h *Hub) BroadcastGroup(groupID string, event Event) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.groups[groupID]))
	for client := range h.groups[groupID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		select {
		case client.Send <- event:
		default:
			h.Unregister(client)
		}
	}
}

func (h *Hub) NotifyUser(userID string, event Event) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.users[userID]))
	for client := range h.users[userID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		select {
		case client.Send <- event:
		default:
			h.Unregister(client)
		}
	}
}

type Client struct {
	Hub     *Hub
	Conn    *websocket.Conn
	Send    chan Event
	UserID  string
	GroupID string
}

func NewClient(hub *Hub, conn *websocket.Conn, user domain.User, groupID string) *Client {
	return &Client{Hub: hub, Conn: conn, Send: make(chan Event, 32), UserID: user.ID, GroupID: groupID}
}

func (c *Client) ReadPump() {
	defer func() { c.Hub.Unregister(c); _ = c.Conn.Close() }()
	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error { _ = c.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() { ticker.Stop(); _ = c.Conn.Close() }()
	for {
		select {
		case event, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				c.Hub.logger.Printf("failed to marshal websocket event: %v", err)
				continue
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
