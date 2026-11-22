package push

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func newTestNotifier(rt roundTripFunc, concurrency int) *FCMNotifier {
	n := &FCMNotifier{
		ProjectID:   "p",
		ClientEmail: "e@example.invalid",
		PrivateKey:  "unused",
		HTTPClient:  &http.Client{Transport: rt},
		Concurrency: concurrency,
	}
	// Pre-seed the OAuth token so the test never signs a JWT.
	n.accessToken = "test-token"
	n.expiresAt = time.Now().Add(time.Hour)
	return n
}

func tokenFromRequest(t *testing.T, r *http.Request) string {
	var payload struct {
		Message struct {
			Token string `json:"token"`
		} `json:"message"`
	}
	data, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Errorf("decode request: %v", err)
	}
	return payload.Message.Token
}

func TestSendToTokensRunsConcurrentlyWithinBound(t *testing.T) {
	var inFlight, maxInFlight atomic.Int64
	var mu sync.Mutex
	sent := map[string]int{}
	n := newTestNotifier(func(r *http.Request) (*http.Response, error) {
		current := inFlight.Add(1)
		for {
			prev := maxInFlight.Load()
			if current <= prev || maxInFlight.CompareAndSwap(prev, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		token := tokenFromRequest(t, r)
		mu.Lock()
		sent[token]++
		mu.Unlock()
		return jsonResponse(200, `{}`), nil
	}, 4)

	tokens := make([]string, 0, 40)
	for i := 0; i < 20; i++ {
		tokens = append(tokens, fmt.Sprintf("tok-%d", i), fmt.Sprintf(" tok-%d ", i)) // duplicates are dropped
	}
	started := time.Now()
	if _, err := n.SendToTokens(context.Background(), tokens, service.PushMessage{Title: "t", Body: "b"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(sent) != 20 {
		t.Fatalf("sent to %d distinct tokens, want 20", len(sent))
	}
	for token, count := range sent {
		if count != 1 {
			t.Fatalf("token %s sent %d times", token, count)
		}
	}
	if got := maxInFlight.Load(); got < 2 || got > 4 {
		t.Fatalf("max in-flight sends = %d, want between 2 and 4", got)
	}
	// 20 sends of 20ms with 4 workers take ~100ms; sequential would take 400ms.
	if elapsed := time.Since(started); elapsed > 350*time.Millisecond {
		t.Fatalf("fan-out was not concurrent: %v", elapsed)
	}
}

func TestSendToTokensKeepsRetryAndInvalidTokenSemantics(t *testing.T) {
	var flakyCalls atomic.Int64
	n := newTestNotifier(func(r *http.Request) (*http.Response, error) {
		switch token := tokenFromRequest(t, r); token {
		case "gone":
			return jsonResponse(404, `{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`), nil
		case "flaky":
			if flakyCalls.Add(1) < 3 {
				return jsonResponse(503, `unavailable`), nil
			}
			return jsonResponse(200, `{}`), nil
		case "broken":
			return jsonResponse(403, `PERMISSION_DENIED`), nil
		default:
			return jsonResponse(200, `{}`), nil
		}
	}, 0)

	result, err := n.SendToTokens(context.Background(), []string{"ok", "gone", "flaky", "broken"}, service.PushMessage{})
	if err == nil || !strings.Contains(err.Error(), "1 token(s)") {
		t.Fatalf("expected one permanent failure, got %v", err)
	}
	if len(result.InvalidTokens) != 1 || result.InvalidTokens[0] != "gone" {
		t.Fatalf("invalid tokens = %v, want [gone]", result.InvalidTokens)
	}
	if got := flakyCalls.Load(); got != 3 {
		t.Fatalf("transient token attempts = %d, want 3", got)
	}
}
