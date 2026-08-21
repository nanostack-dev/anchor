package permission

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

// Permission-specific business rule errors.
var (
	ErrPermissionNameDuplicate = fault.Conflict(
		"PERMISSION_NAME_DUPLICATE",
		"A permission with this name already exists in the product",
	)

	ErrPermissionAssignedToRoles = fault.Conflict(
		"PERMISSION_ASSIGNED_TO_ROLES",
		"Cannot delete permission that is assigned to one or more roles",
	)

	ErrPermissionInvalidFormat = fault.BadRequest(
		"PERMISSION_INVALID_FORMAT",
		"Permission name must follow format 'resource:action'",
	)

	ErrProductNotFound = fault.NotFound(
		"PRODUCT_NOT_FOUND",
		"Product does not exist",
	)

	ErrPermissionNotFound = fault.NotFound(
		"PERMISSION_NOT_FOUND",
		"Permission does not exist",
	)
)

// NewPermissionNameDuplicateError creates an error for duplicate permission names.
func NewPermissionNameDuplicateError(
	permissionName string, productID string,
) *fault.Error {
	return fault.Conflict("PERMISSION_NAME_DUPLICATE", "A permission with this name already exists in the product").
		Metadata(map[string]any{
			"permission_name": permissionName,
			"product_id":      productID,
		})
}

func NewPermissionAssignedToAPIKeysError(
	productID, permissionName string, apiKeyCount int,
) *fault.Error {
	return fault.Conflict("PERMISSION_ASSIGNED_TO_PRODUCT_API_KEYS", "Cannot delete permission that is assigned to one or more API keys").
		Metadata(map[string]any{
			"product_id":      productID,
			"permission_name": permissionName,
			"api_key_count":   apiKeyCount,
		})
}
