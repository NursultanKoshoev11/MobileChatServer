package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestBearerTokenFromWebSocketProtocol(t *testing.T) {
	tokenValue := "aaa.bbb.ccc"
	request := httptest.NewRequest("GET", "/api/ws", nil)
	request.Header.Set("Sec-WebSocket-Protocol", "koom-ws, "+tokenValue)

	token, webSocketToken := bearerTokenFromRequest(request)
	if token != tokenValue {
		t.Fatalf("expected websocket protocol token %q, got %q", tokenValue, token)
	}
	if !webSocketToken {
		t.Fatal("expected token to be treated as websocket token")
	}
}

func TestBearerTokenFromWebSocketProtocolOnlyAppliesToWebSocketPaths(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/me", nil)
	request.Header.Set("Sec-WebSocket-Protocol", "koom-ws, aaa.bbb.ccc")

	token, webSocketToken := bearerTokenFromRequest(request)
	if token != "" {
		t.Fatalf("expected no token on non-websocket path, got %q", token)
	}
	if webSocketToken {
		t.Fatal("expected non-websocket path not to use websocket auth")
	}
}
