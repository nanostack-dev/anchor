package role

import (
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

var (
	ErrProductRoleNotFound = fault.NotFound(
		"ROLE_NOT_FOUND",
		"This product has no role with that identifier",
	)

	ErrDefaultNotOrganizationScoped = fault.BadRequest(
		"DEFAULT_ROLE_NOT_ORGANIZATION_SCOPED",
		"A default role is the role a member gets on joining, so it is scoped to organization.",
	)

	ErrScopeImmutable = fault.Conflict(
		"ROLE_SCOPE_IMMUTABLE",
		"A role's resource type can't change. Create a role with the new scope instead.",
	)
)

// NewPermissionOutsideScopeError refuses a bundle with a permission scoped to
// a type that is neither the role's own type nor below it (ADR-0022 D1).
func NewPermissionOutsideScopeError(permission, permissionScope, roleScope string) *fault.Error {
	return fault.BadRequest(
		"PERMISSION_OUTSIDE_ROLE_SCOPE",
		fmt.Sprintf("%s applies to %s, which is not %s or a type below it.", permission, permissionScope, roleScope),
	).Metadata(map[string]any{
		"permission_name":          permission,
		"permission_resource_type": permissionScope,
		"role_resource_type":       roleScope,
	})
}

func NewKeyExistsError(key string) *fault.Error {
	return fault.Conflict(
		"ROLE_KEY_DUPLICATE",
		"This product already has a role template with that key.",
	).Metadata(map[string]any{"key": key})
}
