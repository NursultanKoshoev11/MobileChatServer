package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/gorilla/websocket"
)

const (
	writeWait               = 10 * time.Second
	pongWait                = 60 * time.Second
	pingPeriod              = (pongWait * 9) / 10
	maxMessageSize          = 4096
	sendBufferSize          = 256
	deliveryWait            = 250 * time.Millisecond
	websocketDrain          = 30 * time.Second
	maxTotalConnections     = 10000
	maxConnectionsPerUser   = 20
	maxConnectionsPerIP     = 50
	maxClientMessagesMinute = 120
)

type Event struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	GroupID string `json:"group_id,omitempty"`
	Payload any    `json:"payload,omitempty"`
	SentAt  string `json:"sent_at,omitempty"`
}

type AuthRefreshValidator func(ctx context.Context, token string) (userID string, err error)

type Hub struct {
	logger               *log.Logger
	authRefreshValidator AuthRefreshValidator
	mu                   sync.RWMutex
	groups               map[string]map[*Client]bool
	users                map[string]map[*Client]bool
	remoteIPs            map[string]map[*Client]bool
	eventCounter         uint64
}

func NewHub(logger *log.Logger, validators ...AuthRefreshValidator) *Hub {
	var validator AuthRefreshValidator
	if len(validators) > 0 {
		validator = validators[0]
	}
	return &Hub{
		logger:               logger,
		authRefreshValidator: validator,
		groups:               make(map[string]map[*Client]bool),
		users:                make(map[string]map[*Client]bool),
		remoteIPs:            make(map[string]map[*Client]bool),
	}
}

func (h *Hub) Register(client *Client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clientCountLocked() >= maxTotalConnections {
		h.logger.Printf("websocket rejected: total connection limit reached user_id=%s remote_ip=%s", client.UserID, client.RemoteIP)
		return false
	}
	if len(h.users[client.UserID]) >= maxConnectionsPerUser {
		h.logger.Printf("websocket rejected: per-user connection limit reached user_id=%s remote_ip=%s", client.UserID, client.RemoteIP)
		return false
	}
	if client.RemoteIP != "" && len(h.remoteIPs[client.RemoteIP]) >= maxConnectionsPerIP {
		h.logger.Printf("websocket rejected: per-ip connection limit reached user_id=%s remote_ip=%s", client.UserID, client.RemoteIP)
		return false
	}
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
	if client.RemoteIP != "" {
		if h.remoteIPs[client.RemoteIP] == nil {
			h.remoteIPs[client.RemoteIP] = make(map[*Client]bool)
		}
		h.remoteIPs[client.RemoteIP][client] = true
	}
	return true
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	closed := false
	if client.GroupID != "" {
		if clients := h.groups[client.GroupID]; clients != nil {
			if clients[client] {
				delete(clients, client)
				client.closeSend()
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
				client.closeSend()
			}
		}
		if len(clients) == 0 {
			delete(h.users, client.UserID)
		}
	}
	if client.RemoteIP != "" {
		if clients := h.remoteIPs[client.RemoteIP]; clients != nil {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.remoteIPs, client.RemoteIP)
			}
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
		h.deliver(client, event)
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
		h.deliver(client, event)
	}
}

func (h *Hub) deliver(client *Client, event Event) {
	event = h.prepareEvent(event)
	if client.sendEvent(event) {
		return
	}
	h.logger.Printf("websocket delivery dropped type=%s group_id=%s user_id=%s queued=%d", event.Type, client.GroupID, client.UserID, len(client.Send))
	h.Unregister(client)
}

func (h *Hub) Drain(timeout time.Duration) {
	if timeout <= 0 {
		timeout = websocketDrain
	}
	deadline := time.Now().Add(timeout)
	for {
		if h.ClientCount() == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.CloseAll()
}

func (h *Hub) CloseAll() {
	h.mu.Lock()
	clients := make([]*Client, 0)
	for _, groupClients := range h.groups {
		for client := range groupClients {
			clients = append(clients, client)
		}
	}
	for _, userClients := range h.users {
		for client := range userClients {
			clients = append(clients, client)
		}
	}
	h.mu.Unlock()
	seen := map[*Client]bool{}
	for _, client := range clients {
		if seen[client] {
			continue
		}
		seen[client] = true
		h.Unregister(client)
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clientCountLocked()
}

func (h *Hub) clientCountLocked() int {
	seen := map[*Client]bool{}
	for _, groupClients := range h.groups {
		for client := range groupClients {
			seen[client] = true
		}
	}
	for _, userClients := range h.users {
		for client := range userClients {
			seen[client] = true
		}
	}
	return len(seen)
}

func (h *Hub) prepareEvent(event Event) Event {
	if event.ID == "" {
		seq := atomic.AddUint64(&h.eventCounter, 1)
		event.ID = fmt.Sprintf("rt-%d-%d", time.Now().UnixNano(), seq)
	}
	if event.SentAt == "" {
		event.SentAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return event
}

type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan Event
	UserID   string
	GroupID  string
	RemoteIP string

	sendMu     sync.RWMutex
	sendClosed bool

	readWindowStart time.Time
	readCount       int
}

func NewClient(hub *Hub, conn *websocket.Conn, user domain.User, groupID string, remoteIP string) *Client {
	return &Client{Hub: hub, Conn: conn, Send: make(chan Event, sendBufferSize), UserID: user.ID, GroupID: groupID, RemoteIP: remoteIP}
}

func (c *Client) sendEvent(event Event) bool {
	c.sendMu.RLock()
	defer c.sendMu.RUnlock()
	if c.sendClosed {
		return false
	}
	select {
	case c.Send <- event:
		return true
	default:
		return false
	}
}

func (c *Client) closeSend() {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.sendClosed {
		return
	}
	close(c.Send)
	c.sendClosed = true
}

func (c *Client) ReadPump() {
	defer func() { c.Hub.Unregister(c); _ = c.Conn.Close() }()
	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error { _ = c.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, payload, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		if !c.allowClientMessage() {
			c.Hub.logger.Printf("websocket client message rate limit user_id=%s remote_ip=%s", c.UserID, c.RemoteIP)
			break
		}
		if !c.handleClientMessage(payload) {
			break
		}
	}
}

func (c *Client) allowClientMessage() bool {
	now := time.Now()
	if c.readWindowStart.IsZero() || now.Sub(c.readWindowStart) >= time.Minute {
		c.readWindowStart = now
		c.readCount = 1
		return true
	}
	c.readCount++
	return c.readCount <= maxClientMessagesMinute
}

func (c *Client) handleClientMessage(payload []byte) bool {
	var message struct {
		Type    string `json:"type"`
		EventID string `json:"event_id"`
		Reason  string `json:"reason"`
		Token   string `json:"token"`
	}
	if err := json.Unmarshal(payload, &message); err != nil {
		return true
	}
	switch message.Type {
	case "ack":
		c.Hub.logger.Printf("websocket ack event_id=%s user_id=%s group_id=%s", message.EventID, c.UserID, c.GroupID)
	case "nack":
		c.Hub.logger.Printf("websocket nack event_id=%s user_id=%s group_id=%s reason=%s", message.EventID, c.UserID, c.GroupID, message.Reason)
	case "ping":
		c.sendEvent(Event{Type: "pong", Payload: map[string]string{"ts": time.Now().UTC().Format(time.RFC3339Nano)}})
	case "auth_refresh":
		return c.handleAuthRefresh(message.Token)
	}
	return true
}

func (c *Client) handleAuthRefresh(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		c.Hub.logger.Printf("websocket auth refresh rejected: missing token user_id=%s remote_ip=%s", c.UserID, c.RemoteIP)
		return false
	}
	validator := c.Hub.authRefreshValidator
	if validator == nil {
		c.Hub.logger.Printf("websocket auth refresh rejected: validator is not configured user_id=%s remote_ip=%s", c.UserID, c.RemoteIP)
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	claimsCh := make(chan struct {
		userID string
		err    error
	}, 1)
	go func() {
		userID, err := validator(ctx, token)
		claimsCh <- struct {
			userID string
			err    error
		}{userID: userID, err: err}
	}()

	select {
	case <-ctx.Done():
		c.Hub.logger.Printf("websocket auth refresh rejected: validation timeout user_id=%s remote_ip=%s", c.UserID, c.RemoteIP)
		return false
	case result := <-claimsCh:
		if result.err != nil {
			c.Hub.logger.Printf("websocket auth refresh rejected: invalid token user_id=%s remote_ip=%s error=%v", c.UserID, c.RemoteIP, result.err)
			return false
		}
		if result.userID != c.UserID {
			c.Hub.logger.Printf("websocket auth refresh rejected: token user mismatch current_user_id=%s token_user_id=%s remote_ip=%s", c.UserID, result.userID, c.RemoteIP)
			return false
		}
	}

	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.sendEvent(Event{Type: "auth.refreshed", Payload: map[string]string{"user_id": c.UserID}})
	c.Hub.logger.Printf("websocket auth refreshed user_id=%s group_id=%s remote_ip=%s", c.UserID, c.GroupID, c.RemoteIP)
	return true
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
