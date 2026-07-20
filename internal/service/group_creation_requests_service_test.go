package service

import (
	"testing"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func TestGroupRequestReviewCapabilityIncludesPlatformAndSuperAdmin(t *testing.T) {
	if domain.UserRoleUser.CanReviewGroupCreationRequests() {
		t.Fatal("regular user must not review group creation requests")
	}
	if !domain.UserRolePlatformAdmin.CanReviewGroupCreationRequests() {
		t.Fatal("platform admin must review group creation requests")
	}
	if !domain.UserRoleSuperAdmin.CanReviewGroupCreationRequests() {
		t.Fatal("super admin must inherit group request review capability")
	}
}

func TestGlobalGroupDeletionRequiresSuperAdmin(t *testing.T) {
	svc := &Service{}
	if err := svc.DeleteGroupAsPlatformAdmin(t.Context(), domain.User{Role: domain.UserRoleUser}, "G-1"); err == nil {
		t.Fatal("regular user must not delete groups globally")
	}
	if err := svc.DeleteGroupAsPlatformAdmin(t.Context(), domain.User{Role: domain.UserRolePlatformAdmin}, "G-1"); err == nil {
		t.Fatal("platform admin must not delete groups globally")
	}
	if err := svc.DeleteGroupAsPlatformAdmin(t.Context(), domain.User{Role: domain.UserRoleSuperAdmin}, ""); err == nil {
		t.Fatal("super admin deletion still requires a group id")
	}
}
