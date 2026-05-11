package permission

import (
	"net/http"

	"github.com/nanostack-dev/shared/toolkit"
)

// Permission-specific business rule errors.
var (
	ErrPermissionNameDuplicate = toolkit.NewNanostackErrorsWithStatus(
		"PERMISSION_NAME_DUPLICATE",
		"A permission with this name already exists in the product",
		http.StatusConflict,
	)

	ErrPermissionAssignedToRoles = toolkit.NewNanostackErrorsWithStatus(
		"PERMISSION_ASSIGNED_TO_ROLES",
		"Cannot delete permission that is assigned to one or more roles",
		http.StatusConflict,
	)

	ErrPermissionInvalidFormat = toolkit.NewNanostackErrorsWithStatus(
		"PERMISSION_INVALID_FORMAT",
		"Permission name must follow format 'resource:action'",
		http.StatusBadRequest,
	)

	ErrProductNotFound = toolkit.NewNanostackErrorsWithStatus(
		"PRODUCT_NOT_FOUND",
		"Product does not exist",
		http.StatusNotFound,
	)

	ErrPermissionNotFound = toolkit.NewNanostackErrorsWithStatus(
		"PERMISSIONS_NOT_FOUND",
		"Permission does not exist",
		http.StatusNotFound,
	)
)

// NewPermissionNameDuplicateError creates an error for duplicate permission names.
func NewPermissionNameDuplicateError(
	permissionName string, productID string,
) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithMetadata(
		"PERMISSION_NAME_DUPLICATE",
		"A permission with this name already exists in the product",
		map[string]interface{}{
			"permission_name": permissionName,
			"product_id":      productID,
		},
	)
}

func NewPermissionAssignedToAPIKeysError(
	productID, permissionName string, apiKeyCount int,
) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithMetadata(
		"PERMISSION_ASSIGNED_TO_PRODUCT_API_KEYS",
		"Cannot delete permission that is assigned to one or more API keys",
		map[string]interface{}{
			"product_id":      productID,
			"permission_name": permissionName,
			"api_key_count":   apiKeyCount,
		},
	)
}
