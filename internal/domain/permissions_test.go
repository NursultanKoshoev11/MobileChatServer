package domain

import "testing"

func TestPlatformRoleCapabilitiesAreSeparated(t *testing.T) {
	tests := []struct {
		name            string
		role            UserRole
		isPlatformAdmin bool
		isSuperAdmin    bool
		canReview       bool
		canManageGroups bool
		canModerateAny  bool
	}{
		{
			name: "regular user",
			role: UserRoleUser,
		},
		{
			name:            "platform admin",
			role:            UserRolePlatformAdmin,
			isPlatformAdmin: true,
			canReview:       true,
		},
		{
			name:            "super admin",
			role:            UserRoleSuperAdmin,
			isSuperAdmin:    true,
			canReview:       true,
			canManageGroups: true,
			canModerateAny:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.role.IsPlatformAdmin(); got != test.isPlatformAdmin {
				t.Fatalf("IsPlatformAdmin() = %v, want %v", got, test.isPlatformAdmin)
			}
			if got := test.role.IsSuperAdmin(); got != test.isSuperAdmin {
				t.Fatalf("IsSuperAdmin() = %v, want %v", got, test.isSuperAdmin)
			}
			if got := test.role.CanReviewGroupCreationRequests(); got != test.canReview {
				t.Fatalf("CanReviewGroupCreationRequests() = %v, want %v", got, test.canReview)
			}
			if got := test.role.CanManageAllGroups(); got != test.canManageGroups {
				t.Fatalf("CanManageAllGroups() = %v, want %v", got, test.canManageGroups)
			}
			if got := test.role.CanModerateAnyGroup(); got != test.canModerateAny {
				t.Fatalf("CanModerateAnyGroup() = %v, want %v", got, test.canModerateAny)
			}
		})
	}
}
