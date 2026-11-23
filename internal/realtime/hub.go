package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
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
	revokedUsers         map[string]struct{}
	eventCounter         uint64
	// shuttingDown rejects new registrations and makes write pumps send a
	// "going away" close frame.
	shuttingDown atomic.Bool
	// activeWriters counts running WritePump goroutines (one per socket).
	activeWriters atomic.Int64
}

// remoteIPKey groups IPv6 clients by their /64 prefix (one subscriber usually
// owns a whole /64), matching the HTTP rate limiter. IPv4 addresses and
// unparsable values are used as-is.
func remoteIPKey(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil || ip.To4() != nil {
		return raw
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
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
		revokedUsers:         make(map[string]struct{}),
	}
}

func (h *Hub) Register(client *Client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.shuttingDown.Load() {
		h.logger.Printf("websocket rejected: server is shutting down user_id=%q remote_ip=%q", client.UserID, client.RemoteIP)
		return false
	}
	if _, revoked := h.revokedUsers[client.UserID]; revoked {
		h.logger.Printf("websocket rejected: account was deleted user_id=%q remote_ip=%q", client.UserID, client.RemoteIP)
		return false
	}
	if h.clientCountLocked() >= maxTotalConnections {
		h.logger.Printf("websocket rejected: total connection limit reached user_id=%q remote_ip=%q", client.UserID, client.RemoteIP)
		return false
	}
	if len(h.users[client.UserID]) >= maxConnectionsPerUser {
		h.logger.Printf("websocket rejected: per-user connection limit reached user_id=%q remote_ip=%q", client.UserID, client.RemoteIP)
		return false
	}
	ipKey := remoteIPKey(client.RemoteIP)
	if client.RemoteIP != "" && len(h.remoteIPs[ipKey]) >= maxConnectionsPerIP {
		h.logger.Printf("websocket rejected: per-ip connection limit reached user_id=%q remote_ip=%q", client.UserID, client.RemoteIP)
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
		if h.remoteIPs[ipKey] == nil {
			h.remoteIPs[ipKey] = make(map[*Client]bool)
		}
		h.remoteIPs[ipKey][client] = true
	}
	return true
}

// DisconnectUser revokes this process's existing and in-flight WebSocket
// registrations after account deletion. Deleted user IDs cannot authenticate
// again, so retaining the in-memory tombstone until process restart is safe.
func (h *Hub) DisconnectUser(userID string) {
	h.mu.Lock()
	h.revokedUsers[userID] = struct{}{}
	clients := make([]*Client, 0, len(h.users[userID]))
	for client := range h.users[userID] {
		clients = append(clients, client)
	}
	h.mu.Unlock()

	for _, client := range clients {
		h.Unregister(client)
		_ = client.Conn.Close()
	}
}

// DisconnectUserSessions closes this process's sockets for userID without
// blocking future connections (used on logout; the user may sign in again).
func (h *Hub) DisconnectUserSessions(userID string) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.users[userID]))
	for client := range h.users[userID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	h.closeClients(clients)
}

// RemoveUserFromGroup stops delivering a group's events to a user who left or
// was removed from it by closing that user's sockets subscribed to the group.
// Group events sent through user sockets are addressed from the current member
// list, so those need no change.
func (h *Hub) RemoveUserFromGroup(groupID, userID string) {
	h.mu.RLock()
	clients := make([]*Client, 0)
	for client := range h.groups[groupID] {
		if client.UserID == userID {
			clients = append(clients, client)
		}
	}
	h.mu.RUnlock()
	h.closeClients(clients)
}

// CloseGroup closes every socket subscribed to a deleted group.
func (h *Hub) CloseGroup(groupID string) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.groups[groupID]))
	for client := range h.groups[groupID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	h.closeClients(clients)
}

func (h *Hub) closeClients(clients []*Client) {
	for _, client := range clients {
		h.Unregister(client)
		if client.Conn != nil {
			_ = client.Conn.Close()
		}
	}
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
		ipKey := remoteIPKey(client.RemoteIP)
		if clients := h.remoteIPs[ipKey]; clients != nil {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.remoteIPs, ipKey)
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

// BroadcastGroupAndUsers delivers event exactly once to every socket that is
// subscribed to groupID or belongs to one of userIDs. A user's group socket is
// also registered as one of that user's sockets, so delivering to the group and
// then to each user separately would send the same event twice. Sockets of
// skipUserIDs receive nothing (for example members who blocked the author).
func (h *Hub) BroadcastGroupAndUsers(groupID string, userIDs []string, skipUserIDs []string, event Event) {
	skip := make(map[string]bool, len(skipUserIDs))
	for _, userID := range skipUserIDs {
		skip[userID] = true
	}
	h.mu.RLock()
	seen := make(map[*Client]bool)
	clients := make([]*Client, 0, len(h.groups[groupID]))
	add := func(client *Client) {
		if seen[client] || skip[client.UserID] {
			return
		}
		seen[client] = true
		clients = append(clients, client)
	}
	for client := range h.groups[groupID] {
		add(client)
	}
	for _, userID := range userIDs {
		for client := range h.users[userID] {
			add(client)
		}
	}
	h.mu.RUnlock()
	event = h.prepareEvent(event)
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
	h.logger.Printf("websocket delivery dropped type=%q group_id=%q user_id=%q queued=%d", event.Type, client.GroupID, client.UserID, len(client.Send))
	h.Unregister(client)
}

// Drain is used on graceful shutdown. It stops accepting new sockets, sends
// every connected client a "going away" close frame (clients reconnect to the
// next instance), waits up to timeout for the write pumps to flush it, and then
// force-closes whatever is left.
func (h *Hub) Drain(timeout time.Duration) {
	if timeout <= 0 {
		timeout = websocketDrain
	}
	h.shuttingDown.Store(true)
	clients := h.CloseAll()
	deadline := time.Now().Add(timeout)
	for h.activeWriters.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	for _, client := range clients {
		if client.Conn != nil {
			_ = client.Conn.Close()
		}
	}
}

// CloseAll unregisters every client, which makes each write pump send a close
// frame and close its connection. It returns the clients it closed.
func (h *Hub) CloseAll() []*Client {
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
	closed := make([]*Client, 0, len(seen))
	for client := range seen {
		closed = append(closed, client)
	}
	return closed
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
			c.Hub.logger.Printf("websocket client message rate limit user_id=%q remote_ip=%q", c.UserID, c.RemoteIP)
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
		// Acknowledgements are frequent and client-controlled; keep them out of logs.
	case "nack":
		c.Hub.logger.Printf("websocket nack user_id=%q group_id=%q reason_bytes=%d", c.UserID, c.GroupID, len(message.Reason))
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
		c.Hub.logger.Printf("websocket auth refresh rejected: missing token user_id=%q remote_ip=%q", c.UserID, c.RemoteIP)
		return false
	}
	validator := c.Hub.authRefreshValidator
	if validator == nil {
		c.Hub.logger.Printf("websocket auth refresh rejected: validator is not configured user_id=%q remote_ip=%q", c.UserID, c.RemoteIP)
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
		c.Hub.logger.Printf("websocket auth refresh rejected: validation timeout user_id=%q remote_ip=%q", c.UserID, c.RemoteIP)
		return false
	case result := <-claimsCh:
		if result.err != nil {
			c.Hub.logger.Printf("websocket auth refresh rejected: invalid token user_id=%q remote_ip=%q error=%q", c.UserID, c.RemoteIP, fmt.Sprint(result.err))
			return false
		}
		if result.userID != c.UserID {
			c.Hub.logger.Printf("websocket auth refresh rejected: token user mismatch current_user_id=%q token_user_id=%q remote_ip=%q", c.UserID, result.userID, c.RemoteIP)
			return false
		}
	}

	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.sendEvent(Event{Type: "auth.refreshed", Payload: map[string]string{"user_id": c.UserID}})
	c.Hub.logger.Printf("websocket auth refreshed user_id=%q group_id=%q remote_ip=%q", c.UserID, c.GroupID, c.RemoteIP)
	return true
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	c.Hub.activeWriters.Add(1)
	defer func() { ticker.Stop(); _ = c.Conn.Close(); c.Hub.activeWriters.Add(-1) }()
	for {
		select {
		case event, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				closeFrame := []byte{}
				if c.Hub.shuttingDown.Load() {
					closeFrame = websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down")
				}
				_ = c.Conn.WriteMessage(websocket.CloseMessage, closeFrame)
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
