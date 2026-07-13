package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
)

func TestRefreshErrorContractUnauthorizedIs401(t *testing.T) {
	server := &Server{logger: log.New(io.Discard, "", 0)}
	recorder := httptest.NewRecorder()

	server.writeError(recorder, service.ErrUnauthorized)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, recorder.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "unauthorized" {
		t.Fatalf("expected unauthorized error, got %q", payload["error"])
	}
}

func TestRefreshErrorContractTemporaryFailureRemains5xx(t *testing.T) {
	server := &Server{logger: log.New(io.Discard, "", 0)}

	t.Run("service unavailable", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		server.writeError(recorder, service.NewServiceUnavailableError("temporarily unavailable"))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
		}
	})

	t.Run("internal failure", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		server.writeError(recorder, errors.New("database connection interrupted"))
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, recorder.Code)
		}
	})
}
