package permission

import (
	"net/http"

	apierror "github.com/nanostack-dev/nanostack-framework/pkg/apierror"
)

// Permission-specific business rule errors.
var (
	ErrPermissionNameDuplicate = apierror.NewWithStatus(
		"PERMISSION_NAME_DUPLICATE",
		"A permission with this name already exists in the product",
		http.StatusConflict,
	)

	ErrPermissionAssignedToRoles = apierror.NewWithStatus(
		"PERMISSION_ASSIGNED_TO_ROLES",
		"Cannot delete permission that is assigned to one or more roles",
		http.StatusConflict,
	)

	ErrPermissionInvalidFormat = apierror.NewWithStatus(
		"PERMISSION_INVALID_FORMAT",
		"Permission name must follow format 'resource:action'",
		http.StatusBadRequest,
	)

	ErrProductNotFound = apierror.NewWithStatus(
		"PRODUCT_NOT_FOUND",
		"Product does not exist",
		http.StatusNotFound,
	)

	ErrPermissionNotFound = apierror.NewWithStatus(
		"PERMISSIONS_NOT_FOUND",
		"Permission does not exist",
		http.StatusNotFound,
	)
)

// NewPermissionNameDuplicateError creates an error for duplicate permission names.
func NewPermissionNameDuplicateError(
	permissionName string, productID string,
) *apierror.Error {
	return apierror.NewBadRequestWithMetadata(
		"PERMISSION_NAME_DUPLICATE",
		"A permission with this name already exists in the product",
		map[string]any{
			"permission_name": permissionName,
			"product_id":      productID,
		},
	)
}

func NewPermissionAssignedToAPIKeysError(
	productID, permissionName string, apiKeyCount int,
) *apierror.Error {
	return apierror.NewBadRequestWithMetadata(
		"PERMISSION_ASSIGNED_TO_PRODUCT_API_KEYS",
		"Cannot delete permission that is assigned to one or more API keys",
		map[string]any{
			"product_id":      productID,
			"permission_name": permissionName,
			"api_key_count":   apiKeyCount,
		},
	)
}
