package service

import (
	"testing"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func TestPlatformAdminHierarchyIncludesSuperAdmin(t *testing.T) {
	if isPlatformAdmin(domain.User{Role: domain.UserRoleUser}) {
		t.Fatal("regular user must not have platform administration access")
	}
	if !isPlatformAdmin(domain.User{Role: domain.UserRolePlatformAdmin}) {
		t.Fatal("platform admin must have platform administration access")
	}
	if !isPlatformAdmin(domain.User{Role: domain.UserRoleSuperAdmin}) {
		t.Fatal("super admin must inherit platform administration access")
	}
}

func TestDeleteGroupAsPlatformAdminRequiresPlatformRoleAndGroupID(t *testing.T) {
	svc := &Service{}
	if err := svc.DeleteGroupAsPlatformAdmin(t.Context(), domain.User{Role: domain.UserRoleUser}, "G-1"); err == nil {
		t.Fatal("regular user must not delete groups")
	}
	if err := svc.DeleteGroupAsPlatformAdmin(t.Context(), domain.User{Role: domain.UserRoleSuperAdmin}, ""); err == nil {
		t.Fatal("super admin deletion still requires a group id")
	}
}
