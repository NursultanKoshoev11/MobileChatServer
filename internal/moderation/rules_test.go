package moderation

import "testing"

func TestRuleCheckerBlocksPlainKyrgyzAdvertising(t *testing.T) {
	checker := NewRuleChecker()
	decision := checker.Check(Input{Body: "тоок сатылат"})
	if decision.Action != ActionBlock {
		t.Fatalf("expected block, got %s", decision.Action)
	}
}

func TestRuleCheckerAllowsNormalKyrgyzText(t *testing.T) {
	checker := NewRuleChecker()
	decision := checker.Check(Input{Body: "Саламатсызбы, айылдагы парк боюнча сунуш бар."})
	if decision.Action != ActionAllow {
		t.Fatalf("expected allow, got %s reasons=%v", decision.Action, decision.Reasons)
	}
}

func TestRuleCheckerBlocksProhibitedKeyword(t *testing.T) {
	checker := NewRuleChecker()
	decision := checker.Check(Input{Body: "18+ promo"})
	if decision.Action != ActionBlock {
		t.Fatalf("expected block, got %s reasons=%v", decision.Action, decision.Reasons)
	}
}

func TestRuleCheckerReviewsSuspiciousButNonProhibitedText(t *testing.T) {
	checker := NewRuleChecker()
	decision := checker.Check(Input{Body: "OOOOOOOOO check this please"})
	if decision.Action != ActionReview {
		t.Fatalf("expected review, got %s reasons=%v", decision.Action, decision.Reasons)
	}
}
