package service

import (
	"fmt"
	"net/http"

	"github.com/nanostack-dev/shared/toolkit"
)

const (
	errorDetailName      = fieldNameKey
	errorDetailProductID = fieldProductIDKey
	errorDetailAPIKeyID  = fieldAPIKeyIDKey
)

// From auth/errors.go

var (
	// ErrInvalidCredentials for incorrect email or password.
	ErrInvalidCredentials = toolkit.NewNanostackErrorsWithStatus(
		"EMAIL_OR_PASSWORD_INCORRECT",
		"Email or password is incorrect",
		http.StatusBadRequest,
	)

	// ErrUserAlreadyExists when trying to register an existing email.
	ErrUserAlreadyExists = toolkit.NewNanostackErrorsWithStatus(
		"USER_ALREADY_EXISTS",
		"User with this email already exists",
		http.StatusBadRequest,
	)

	// ErrInvitationCodeNotProvided when a registration attempt lacks an invitation code.
	ErrInvitationCodeNotProvided = toolkit.NewNanostackErrorsWithStatus(
		"INVITATION_CODE_NOT_PROVIDED",
		"Invitation code is required",
		http.StatusBadRequest,
	)

	ErrInvitationCodeIsInvalid = toolkit.NewNanostackErrorsWithStatus(
		"INVITATION_CODE_IS_INVALID",
		"Invitation code is invalid",
		http.StatusBadRequest,
	)

	// ErrUserNotFound when a user lookup fails (often masked by ErrInvalidCredentials).
	ErrUserNotFound = toolkit.NewNanostackErrorsWithStatus(
		"USER_NOT_FOUND",
		"User not found",
		http.StatusNotFound,
	)

	// ErrTokenRefreshFailed for issues refreshing JWTs.
	ErrTokenRefreshFailed = toolkit.NewNanostackErrorsWithStatus(
		"TOKEN_REFRESH_FAILED",
		"Failed to refresh authentication token",
		http.StatusUnauthorized,
	)

	ErrProductAlreadyExists = toolkit.NewNanostackBadRequestError(
		"PRODUCT_ALREADY_EXISTS", "A product with this name already exists in your tenant",
	)

	ErrOwnerRoleNotAllowed = toolkit.NewNanostackErrorsWithStatus(
		"INVITATION_OWNER_ROLE_NOT_ALLOWED",
		"Invitation with OWNER role is not allowed", http.StatusBadRequest,
	)
)

// From resource_permission/errors.go

func NewResourcePermissionInUseError(resourcePermissionID string, roleCount int) error {
	return toolkit.NewNanostackErrorsWithMetadata(
		"RESOURCE_PERMISSION_IN_USE",
		fmt.Sprintf(
			"Resource permission %s cannot be deleted because it is assigned to %d role(s)",
			resourcePermissionID, roleCount,
		),
		map[string]interface{}{
			"resource_permission_id": resourcePermissionID,
			"role_count":             roleCount,
		},
	)
}

func NewResourcePermissionAlreadyExistsError(name string) error {
	return toolkit.NewNanostackErrorsWithMetadata(
		"RESOURCE_PERMISSION_ALREADY_EXISTS",
		fmt.Sprintf("Resource permission with name '%s' already exists", name),
		map[string]interface{}{
			errorDetailName: name,
		},
	)
}

// From role/errors.go

func NewRoleNotFoundError(roleID string) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithStatus(
		"ROLE_NOT_FOUND",
		fmt.Sprintf("Product role %s does not exist", roleID),
		http.StatusNotFound,
	)
}

func NewRoleWithAlreadyExistingNameError(roleName, productID string) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithMetadata(
		"ROLE_NAME_DUPLICATE",
		"Product role with this name already exists in the product",
		map[string]interface{}{
			"role_name":          roleName,
			errorDetailProductID: productID,
		},
	)
}

func NewPermissionAlreadyAssignedError(roleID, permissionName string) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithMetadata(
		"PERMISSION_ALREADY_ASSIGNED",
		"Permission is already assigned to role",
		map[string]interface{}{
			"role_id":         roleID,
			"permission_name": permissionName,
		},
	)
}

func NewRoleInUseError(roleID string) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithStatus(
		"ROLE_IN_USE",
		fmt.Sprintf("Product role %s cannot be deleted because it is assigned to users", roleID),
		http.StatusBadRequest,
	)
}

// From product/apikey/errors.go

var ErrInvalidAPIKey = toolkit.NewNanostackErrorsWithStatus(
	"INVALID_PRODUCT_API_KEY",
	"Product API key is invalid",
	http.StatusUnauthorized,
)

func NewProductAPIKeyNameExistsError(name, productID string) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithMetadata(
		"PRODUCT_API_KEY_NAME_DUPLICATE",
		"Product API key with this name already exists in the product",
		map[string]interface{}{
			errorDetailName:      name,
			errorDetailProductID: productID,
		},
	)
}

func NewOrganizationAPIKeyNameExistsError(
	name, organizationID string,
) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithMetadata(
		"ORGANIZATION_API_KEY_NAME_DUPLICATE",
		"Organization API key with this name already exists in the organization",
		map[string]interface{}{
			errorDetailName:   name,
			"organization_id": organizationID,
		},
	)
}

func NewOrganizationAPIKeyInactiveOrExpiredError(apiKeyID string) *toolkit.NanostackError {
	return toolkit.NewNanostackErrors(
		"ORGANIZATION_API_KEY_INACTIVE_OR_EXPIRED",
		"Organization API key is inactive or expired",
		map[string]interface{}{
			errorDetailAPIKeyID: apiKeyID,
		},
		http.StatusForbidden,
	)
}

func NewOrganizationAPIKeyExpiresAtInPastError() *toolkit.NanostackError {
	return toolkit.NewNanostackErrors(
		"ORGANIZATION_API_KEY_EXPIRES_AT_IN_PAST",
		"Organization API key expiration date must be in the future",
		map[string]interface{}{},
		http.StatusBadRequest,
	)
}

func NewProductAPIKeyInactiveError(apiKeyID string) *toolkit.NanostackError {
	return toolkit.NewNanostackErrors(
		"PRODUCT_API_KEY_INACTIVE",
		"Product API key is inactive",
		map[string]interface{}{
			errorDetailAPIKeyID: apiKeyID,
		},
		http.StatusForbidden,
	)
}

func NewProductAPIKeyInsufficientPermissionsError(
	apiKeyID string, requiredScopes []string, currentScopes []string,
) *toolkit.NanostackError {
	return toolkit.NewNanostackErrors(
		"PRODUCT_API_KEY_INSUFFICIENT_PERMISSIONS",
		"Product API key does not have sufficient permissions",
		map[string]interface{}{
			errorDetailAPIKeyID: apiKeyID,
			"required_scopes":   requiredScopes,
			"current_scopes":    currentScopes,
		},
		http.StatusForbidden,
	)
}

func NewProductAPIKeyPermissionsImmutableError(apiKeyID string) *toolkit.NanostackError {
	return toolkit.NewNanostackErrors(
		"PRODUCT_API_KEY_PERMISSIONS_IMMUTABLE",
		"Product API key permissions are immutable",
		map[string]interface{}{
			errorDetailAPIKeyID: apiKeyID,
		},
		http.StatusBadRequest,
	)
}

func NewPermissionsNotFoundError(
	productID string, permissionNames []string,
) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithMetadata(
		"PERMISSIONS_NOT_FOUND",
		"Product permission does not exist",
		map[string]interface{}{
			errorDetailProductID: productID,
			"permission_names":   permissionNames,
		},
	)
}

func NewOrganizationMembershipAlreadyExistsError(
	productUserID string,
	organizationID string,
) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithStatus(
		"ORGANIZATION_MEMBERSHIP_ALREADY_EXISTS",
		fmt.Sprintf(
			"Organization membership already exists for product user %s in organization %s",
			productUserID,
			organizationID,
		),
		http.StatusConflict,
	)
}

func NewOrganizationMembershipNotFoundError(
	productUserID string,
	organizationID string,
) *toolkit.NanostackError {
	return toolkit.NewNanostackErrorsWithStatus(
		"ORGANIZATION_MEMBERSHIP_NOT_FOUND",
		fmt.Sprintf(
			"Organization membership not found for product user %s in organization %s",
			productUserID,
			organizationID,
		),
		http.StatusNotFound,
	)
}
