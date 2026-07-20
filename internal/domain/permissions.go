package domain

// Platform capabilities are intentionally defined in one place so new roles or
// privileges can be added without scattering raw role comparisons across the
// API, services, and clients.
func (role UserRole) IsPlatformAdmin() bool {
	return role == UserRolePlatformAdmin
}

func (role UserRole) IsSuperAdmin() bool {
	return role == UserRoleSuperAdmin
}

// CanReviewGroupCreationRequests is the only platform-wide capability granted
// to platform_admin. The project owner (super_admin) inherits this capability.
func (role UserRole) CanReviewGroupCreationRequests() bool {
	return role.IsPlatformAdmin() || role.IsSuperAdmin()
}

// CanManageAllGroups is reserved for the project owner.
func (role UserRole) CanManageAllGroups() bool {
	return role.IsSuperAdmin()
}

// CanModerateAnyGroup is reserved for the project owner. Group owners and group
// admins keep their normal group-scoped moderation rights separately.
func (role UserRole) CanModerateAnyGroup() bool {
	return role.IsSuperAdmin()
}
