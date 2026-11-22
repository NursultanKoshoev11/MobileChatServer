package realtime

import (
	"errors"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/gorilla/websocket"
)

func TestPerIPLimitGroupsIPv6By64(t *testing.T) {
	hub := NewHub(log.New(io.Discard, "", 0))
	for i := 0; i < maxConnectionsPerIP; i++ {
		// Different interface identifiers inside one /64.
		ip := fmt.Sprintf("2001:db8:1:2::%x", i+1)
		if !hub.Register(NewClient(hub, nil, domain.User{ID: fmt.Sprintf("u-%d", i)}, "", ip)) {
			t.Fatalf("registration %d rejected early", i)
		}
	}
	if hub.Register(NewClient(hub, nil, domain.User{ID: "u-extra"}, "", "2001:db8:1:2:ffff:ffff:ffff:ffff")) {
		t.Fatal("a new address in the same /64 bypassed the per-IP websocket cap")
	}
	if !hub.Register(NewClient(hub, nil, domain.User{ID: "u-other"}, "", "2001:db8:1:3::1")) {
		t.Fatal("a different /64 must not share the cap")
	}
	if !hub.Register(NewClient(hub, nil, domain.User{ID: "u-v4"}, "", "192.0.2.10")) {
		t.Fatal("IPv4 clients must keep per-address limits")
	}
}

func TestDrainSendsGoingAwayAndRejectsNewSockets(t *testing.T) {
	hub := NewHub(log.New(io.Discard, "", 0))
	server := newTestWebSocketServer(t, hub, "user-1")
	defer server.Close()

	conn := dialTestWebSocket(t, server.URL)
	defer conn.Close()
	readRealtimeEvent(t, conn, "connection.ready")

	done := make(chan struct{})
	go func() {
		hub.Drain(2 * time.Second)
		close(done)
	}()

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			continue
		}
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) || closeErr.Code != websocket.CloseGoingAway {
			t.Fatalf("expected a going-away close frame, got %v", err)
		}
		break
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Drain did not return")
	}
	if hub.ClientCount() != 0 {
		t.Fatalf("clients left after drain: %d", hub.ClientCount())
	}
	if hub.Register(NewClient(hub, nil, domain.User{ID: "late"}, "", "192.0.2.1")) {
		t.Fatal("new sockets must be rejected while shutting down")
	}
}
