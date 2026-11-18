package realtime

import (
	"io"
	"log"
	"testing"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func newTestClient(hub *Hub, userID, groupID string) *Client {
	client := NewClient(hub, nil, domain.User{ID: userID}, groupID, "")
	if !hub.Register(client) {
		panic("register test client")
	}
	return client
}

// drain returns the event types queued for a client and whether its send
// channel was closed (the client was disconnected).
func drain(client *Client) (types []string, closed bool) {
	for {
		select {
		case event, ok := <-client.Send:
			if !ok {
				return types, true
			}
			types = append(types, event.Type)
		default:
			return types, false
		}
	}
}

func TestRemoveUserFromGroupStopsGroupDelivery(t *testing.T) {
	hub := NewHub(log.New(io.Discard, "", 0))
	leaver := newTestClient(hub, "U-leaver", "G-1")
	leaverOtherGroup := newTestClient(hub, "U-leaver", "G-2")
	leaverUserSocket := newTestClient(hub, "U-leaver", "")
	stayer := newTestClient(hub, "U-stayer", "G-1")

	hub.RemoveUserFromGroup("G-1", "U-leaver")
	hub.BroadcastGroup("G-1", Event{Type: "message.created"})
	hub.BroadcastGroup("G-2", Event{Type: "message.created"})

	if types, closed := drain(leaver); !closed || len(types) != 0 {
		t.Fatalf("removed member socket: events=%v closed=%v", types, closed)
	}
	if types, closed := drain(leaverOtherGroup); closed || len(types) != 1 {
		t.Fatalf("other group socket must stay: events=%v closed=%v", types, closed)
	}
	if _, closed := drain(leaverUserSocket); closed {
		t.Fatal("user-level socket must stay open")
	}
	if types, closed := drain(stayer); closed || len(types) != 1 {
		t.Fatalf("remaining member: events=%v closed=%v", types, closed)
	}
	if got := hub.ClientCount(); got != 3 {
		t.Fatalf("client count = %d, want 3", got)
	}
}

func TestCloseGroupDisconnectsAllGroupSockets(t *testing.T) {
	hub := NewHub(log.New(io.Discard, "", 0))
	a := newTestClient(hub, "U-a", "G-1")
	b := newTestClient(hub, "U-b", "G-1")
	other := newTestClient(hub, "U-a", "G-2")

	hub.CloseGroup("G-1")
	hub.BroadcastGroup("G-1", Event{Type: "message.created"})

	for _, client := range []*Client{a, b} {
		if types, closed := drain(client); !closed || len(types) != 0 {
			t.Fatalf("deleted group socket: events=%v closed=%v", types, closed)
		}
	}
	if _, closed := drain(other); closed {
		t.Fatal("sockets of other groups must stay open")
	}
}

func TestDisconnectUserSessionsAllowsReconnect(t *testing.T) {
	hub := NewHub(log.New(io.Discard, "", 0))
	first := newTestClient(hub, "U-1", "")
	group := newTestClient(hub, "U-1", "G-1")
	someoneElse := newTestClient(hub, "U-2", "")

	hub.DisconnectUserSessions("U-1")
	for _, client := range []*Client{first, group} {
		if _, closed := drain(client); !closed {
			t.Fatal("logged-out user's socket must be closed")
		}
	}
	if _, closed := drain(someoneElse); closed {
		t.Fatal("other users must stay connected")
	}
	// Unlike DisconnectUser (account deletion), logout does not block new sessions.
	if !hub.Register(NewClient(hub, nil, domain.User{ID: "U-1"}, "", "")) {
		t.Fatal("user must be able to reconnect after logout")
	}
}
