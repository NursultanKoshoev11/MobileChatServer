package service

import (
	"context"
	"errors"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

// Phone numbers are sensitive in KOOM: knowing that a number belongs to a user
// of a civic app is itself private. These helpers make every lookup by phone
// follow the same rules:
//
//  1. the actor's role in the group is checked BEFORE the phone is looked up,
//     so outsiders cannot use the endpoints as a "is this number registered?"
//     oracle;
//  2. management actions (roles, comment mutes) only resolve numbers of the
//     group's own members;
//  3. every miss returns the same storage.ErrNotFound, so the response does not
//     reveal whether the number exists in KOOM at all.

func (s *Service) requireGroupRole(ctx context.Context, groupID, actorID string, allowed ...domain.GroupRole) error {
	role, err := s.repo.GetMemberRole(ctx, strings.TrimSpace(groupID), actorID)
	if err != nil {
		return err
	}
	for _, candidate := range allowed {
		if role == candidate {
			return nil
		}
	}
	return storage.ErrForbidden
}

func (s *Service) lookupUserByPhone(ctx context.Context, phone string) (domain.User, error) {
	phone = normalizePhone(phone)
	if phone == "" {
		return domain.User{}, NewValidationError("mobile is required")
	}
	user, err := s.repo.GetUserByPhone(ctx, phone)
	if errors.Is(err, storage.ErrNotFound) {
		return domain.User{}, storage.ErrNotFound
	}
	return user, err
}

// resolveGroupMemberByPhone returns the member of groupID with the given phone,
// after confirming the actor holds one of the allowed roles in that group.
func (s *Service) resolveGroupMemberByPhone(ctx context.Context, actorID, groupID, phone string, allowed ...domain.GroupRole) (domain.User, error) {
	if err := s.requireGroupRole(ctx, groupID, actorID, allowed...); err != nil {
		return domain.User{}, err
	}
	user, err := s.lookupUserByPhone(ctx, phone)
	if err != nil {
		return domain.User{}, err
	}
	if _, err := s.repo.GetMemberRole(ctx, strings.TrimSpace(groupID), user.ID); err != nil {
		if errors.Is(err, storage.ErrForbidden) {
			return domain.User{}, storage.ErrNotFound
		}
		return domain.User{}, err
	}
	return user, nil
}

// ResolveInviteTargetByPhone resolves the user to invite into groupID. Invite
// targets are by definition not members yet, so only the actor's right to
// invite (owner or admin) is checked before the lookup.
func (s *Service) ResolveInviteTargetByPhone(ctx context.Context, actorID, groupID, phone string) (domain.User, error) {
	if err := s.requireGroupRole(ctx, groupID, actorID, domain.RoleOwner, domain.RoleAdmin); err != nil {
		return domain.User{}, err
	}
	return s.lookupUserByPhone(ctx, phone)
}
