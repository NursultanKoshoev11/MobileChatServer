package realtime

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/security"
	"github.com/gorilla/websocket"
)

func TestWebSocketAuthRefreshKeepsConnectionOpen(t *testing.T) {
	secret := strings.Repeat("s", 32)
	refreshToken, err := security.SignWebSocketToken("user-1", secret, time.Minute)
	if err != nil {
		t.Fatalf("sign websocket token: %v", err)
	}

	hub := NewHub(log.New(io.Discard, "", 0), func(ctx context.Context, token string) (string, error) {
		claims, err := security.ParseWebSocketToken(token, secret)
		if err != nil {
			return "", err
		}
		return claims.UserID, nil
	})
	server := newTestWebSocketServer(t, hub, "user-1")
	defer server.Close()

	conn := dialTestWebSocket(t, server.URL)
	defer conn.Close()
	readRealtimeEvent(t, conn, "connection.ready")

	if err := conn.WriteJSON(map[string]string{"type": "auth_refresh", "token": refreshToken}); err != nil {
		t.Fatalf("write auth refresh: %v", err)
	}
	readRealtimeEvent(t, conn, "auth.refreshed")

	if err := conn.WriteJSON(map[string]string{"type": "ping"}); err != nil {
		t.Fatalf("write ping after auth refresh: %v", err)
	}
	readRealtimeEvent(t, conn, "pong")
}

func TestWebSocketAuthRefreshRejectsWrongUser(t *testing.T) {
	secret := strings.Repeat("s", 32)
	refreshToken, err := security.SignWebSocketToken("user-2", secret, time.Minute)
	if err != nil {
		t.Fatalf("sign websocket token: %v", err)
	}

	hub := NewHub(log.New(io.Discard, "", 0), func(ctx context.Context, token string) (string, error) {
		claims, err := security.ParseWebSocketToken(token, secret)
		if err != nil {
			return "", err
		}
		return claims.UserID, nil
	})
	server := newTestWebSocketServer(t, hub, "user-1")
	defer server.Close()

	conn := dialTestWebSocket(t, server.URL)
	defer conn.Close()
	readRealtimeEvent(t, conn, "connection.ready")

	if err := conn.WriteJSON(map[string]string{"type": "auth_refresh", "token": refreshToken}); err != nil {
		t.Fatalf("write auth refresh: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected wrong-user auth refresh to close the websocket")
	}
}

func newTestWebSocketServer(t *testing.T, hub *Hub, userID string) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		client := NewClient(hub, conn, domain.User{ID: userID}, "", "127.0.0.1")
		if !hub.Register(client) {
			t.Errorf("register websocket client failed")
			_ = conn.Close()
			return
		}
		client.Send <- Event{Type: "connection.ready"}
		go client.WritePump()
		go client.ReadPump()
	}))
	t.Cleanup(func() {
		hub.CloseAll()
	})
	return server
}

func dialTestWebSocket(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
}

func readRealtimeEvent(t *testing.T, conn *websocket.Conn, eventType string) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket event %s: %v", eventType, err)
	}
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatalf("decode websocket event %s: %v", eventType, err)
	}
	if event.Type != eventType {
		t.Fatalf("expected websocket event %q, got %q", eventType, event.Type)
	}
}
