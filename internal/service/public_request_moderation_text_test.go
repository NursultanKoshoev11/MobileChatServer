package service

import (
	"testing"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

// A client must not be able to choose the text that moderation sees.
func TestPublicRequestPayloadIgnoresClientModerationText(t *testing.T) {
	body := `{"version":1,"text":"real abusive text","moderation_text":"hello"}`

	meta, isPayload, err := parsePublicRequestBodyPayload(body, "G-1")
	if err != nil {
		t.Fatalf("parse payload: %v", err)
	}
	if !isPayload {
		t.Fatal("expected structured payload")
	}
	if meta.ModerationText != "real abusive text" {
		t.Fatalf("moderation text = %q, want the server-extracted text", meta.ModerationText)
	}
}

func TestModerationBodyForInputUsesStoredText(t *testing.T) {
	item := contentModerationItemForTest(`{"version":1,"text":"stored text","moderation_text":"decoy"}`)

	if got := moderationBodyForInput(item); got != "stored text" {
		t.Fatalf("moderation body = %q, want %q", got, "stored text")
	}
}

func contentModerationItemForTest(body string) domain.ContentModerationItem {
	return domain.ContentModerationItem{
		GroupID:     "G-1",
		ContentType: domain.ContentTypePublicRequest,
		Body:        body,
	}
}
