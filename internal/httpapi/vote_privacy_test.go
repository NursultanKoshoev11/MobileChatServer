package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func TestVoteAggregatePayloadHidesVoter(t *testing.T) {
	voteType := "oppose"
	payload := voteAggregatePayload(domain.PublicRequestVoteUpdate{RequestID: "REQ-1", SupportCount: 3, OpposeCount: 2, VoterID: "U-secret", VoteType: &voteType})
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"voter_id", "vote_type", "U-secret", "oppose\""} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("aggregate vote payload leaks %q: %s", forbidden, text)
		}
	}
	// The client requires request_id and both counts (hasCounts) to apply it.
	var decoded map[string]any
	_ = json.Unmarshal(encoded, &decoded)
	if decoded["request_id"] != "REQ-1" || decoded["support_count"] != float64(3) || decoded["oppose_count"] != float64(2) {
		t.Fatalf("aggregate payload missing counts: %s", text)
	}
}
