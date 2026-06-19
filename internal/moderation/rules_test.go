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
