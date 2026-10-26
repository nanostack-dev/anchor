package resourcepermission

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

var (
	ErrResourcePermissionNotFound = fault.NotFound(
		"RESOURCE_PERMISSION_NOT_FOUND",
		"Resource permission does not exist",
	)

	ErrScopeImmutable = fault.Conflict(
		"PERMISSION_SCOPE_IMMUTABLE",
		"A permission's resource type can't change. Create a permission with the new scope, add it to the roles, then delete this one.",
	)
)
