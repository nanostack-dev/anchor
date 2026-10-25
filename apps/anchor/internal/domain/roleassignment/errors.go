package roleassignment

import (
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

var ErrMemberNotFound = fault.NotFound(
	"RESOURCE_MEMBER_NOT_FOUND",
	"This resource has no role assignment for that product user.",
)

// NewSingleRoleError refuses a request naming more roles than the product
// allows on one resource (ADR-0022 D3).
func NewSingleRoleError(count int) *fault.Error {
	return fault.BadRequest(
		"SINGLE_ROLE_PER_RESOURCE",
		"This product allows one role per member per resource. Send exactly one role ID.",
	).Metadata(map[string]any{"max_roles": MaxRolesPerResource, "role_ids": count})
}

// NewScopeMismatchError refuses a role scoped to another type than the
// resource (ADR-0022 D4).
func NewScopeMismatchError(roleID, roleType, resourceType string) *fault.Error {
	return fault.BadRequest(
		"ROLE_SCOPE_MISMATCH",
		fmt.Sprintf("Role %s is scoped to %s, so it can't be held on a %s.", roleID, roleType, resourceType),
	).Metadata(map[string]any{
		"role_id": roleID, "role_resource_type": roleType, "resource_type": resourceType,
	})
}
