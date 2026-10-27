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

	ErrCustomRolesDisabled = fault.BadRequest(
		"CUSTOM_ROLES_DISABLED",
		"This product doesn't allow custom roles on that resource type.",
	)

	ErrRoleNotEditable = fault.Conflict(
		"ROLE_NOT_EDITABLE",
		"The product manages this role. Only the product can change or delete it.",
	)

	ErrCustomRoleLimitReached = fault.Conflict(
		"CUSTOM_ROLE_LIMIT_REACHED",
		fmt.Sprintf("An organization may hold at most %d custom roles.", MaxCustomRoles),
	).Metadata(map[string]any{"max_custom_roles": MaxCustomRoles})

	ErrScopeImmutable = fault.Conflict(
		"ROLE_SCOPE_IMMUTABLE",
		"A role's resource type can't change. Create a role with the new scope instead.",
	)
)

// NewPermissionOutsideScopeError refuses a role whose permission scopes include one scoped to
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
