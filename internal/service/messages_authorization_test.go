package service

import (
	"testing"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func TestCanPublishOfficialMessage(t *testing.T) {
	tests := []struct {
		role domain.GroupRole
		want bool
	}{
		{role: domain.RoleOwner, want: true},
		{role: domain.RoleAdmin, want: true},
		{role: domain.RoleMember, want: false},
		{role: "unknown", want: false},
	}
	for _, test := range tests {
		t.Run(string(test.role), func(t *testing.T) {
			if got := canPublishOfficialMessage(test.role); got != test.want {
				t.Fatalf("canPublishOfficialMessage(%q) = %t, want %t", test.role, got, test.want)
			}
		})
	}
}
