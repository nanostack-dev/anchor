package service

import (
	"fmt"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

const (
	errorDetailName      = fieldNameKey
	errorDetailProductID = fieldProductIDKey
	errorDetailAPIKeyID  = fieldAPIKeyIDKey
)

// From auth/errors.go

var (
	// ErrInvalidCredentials for incorrect email or password.
	ErrInvalidCredentials = fault.NewWithStatus(
		"EMAIL_OR_PASSWORD_INCORRECT",
		"Email or password is incorrect",
		http.StatusBadRequest,
	)

	// ErrUserAlreadyExists when trying to register an existing email.
	ErrUserAlreadyExists = fault.NewWithStatus(
		"USER_ALREADY_EXISTS",
		"User with this email already exists",
		http.StatusBadRequest,
	)

	// ErrInvitationCodeNotProvided when a registration attempt lacks an invitation code.
	ErrInvitationCodeNotProvided = fault.NewWithStatus(
		"INVITATION_CODE_NOT_PROVIDED",
		"Invitation code is required",
		http.StatusBadRequest,
	)

	ErrInvitationCodeIsInvalid = fault.NewWithStatus(
		"INVITATION_CODE_IS_INVALID",
		"Invitation code is invalid",
		http.StatusBadRequest,
	)

	// ErrUserNotFound when a user lookup fails (often masked by ErrInvalidCredentials).
	ErrUserNotFound = fault.NewWithStatus(
		"USER_NOT_FOUND",
		"User not found",
		http.StatusNotFound,
	)

	// ErrTokenRefreshFailed for issues refreshing JWTs.
	ErrTokenRefreshFailed = fault.NewWithStatus(
		"TOKEN_REFRESH_FAILED",
		"Failed to refresh authentication token",
		http.StatusUnauthorized,
	)

	ErrProductAlreadyExists = fault.BadRequest(
		"PRODUCT_ALREADY_EXISTS", "A product with this name already exists in your tenant",
	)

	ErrOwnerRoleNotAllowed = fault.NewWithStatus(
		"INVITATION_OWNER_ROLE_NOT_ALLOWED",
		"Invitation with OWNER role is not allowed", http.StatusBadRequest,
	)
)

// From resource_permission/errors.go

func NewResourcePermissionInUseError(resourcePermissionID string, roleCount int) error {
	return fault.BadRequest("RESOURCE_PERMISSION_IN_USE", fmt.Sprintf(
		"Resource permission %s cannot be deleted because it is assigned to %d role(s)",
		resourcePermissionID, roleCount,
	)).Metadata(map[string]any{
		"resource_permission_id": resourcePermissionID,
		"role_count":             roleCount,
	})
}

func NewResourcePermissionAlreadyExistsError(name string) error {
	return fault.BadRequest("RESOURCE_PERMISSION_ALREADY_EXISTS", fmt.Sprintf("Resource permission with name '%s' already exists", name)).
		Metadata(map[string]any{
			errorDetailName: name,
		})
}

// From role/errors.go

func NewRoleNotFoundError(roleID string) *fault.Error {
	return fault.NewWithStatus(
		"ROLE_NOT_FOUND",
		fmt.Sprintf("Product role %s does not exist", roleID),
		http.StatusNotFound,
	)
}

func NewProductUserNotFoundError(productUserID string) *fault.Error {
	return fault.NewWithStatus(
		"PRODUCT_USER_NOT_FOUND",
		fmt.Sprintf("Product user %s does not exist", productUserID),
		http.StatusNotFound,
	)
}

func NewOrganizationNotFoundError(organizationID string) *fault.Error {
	return fault.NewWithStatus(
		"ORGANIZATION_NOT_FOUND",
		fmt.Sprintf("Organization %s does not exist", organizationID),
		http.StatusNotFound,
	)
}

func NewRoleWithAlreadyExistingNameError(roleName, productID string) *fault.Error {
	return fault.BadRequest("ROLE_NAME_DUPLICATE", "Product role with this name already exists in the product").
		Metadata(map[string]any{
			"role_name":          roleName,
			errorDetailProductID: productID,
		})
}

func NewPermissionAlreadyAssignedError(roleID, permissionName string) *fault.Error {
	return fault.BadRequest("PERMISSION_ALREADY_ASSIGNED", "Permission is already assigned to role").
		Metadata(map[string]any{
			"role_id":         roleID,
			"permission_name": permissionName,
		})
}

func NewRoleInUseError(roleID string) *fault.Error {
	return fault.NewWithStatus(
		"ROLE_IN_USE",
		fmt.Sprintf("Product role %s cannot be deleted because it is assigned to users", roleID),
		http.StatusBadRequest,
	)
}

// From product/apikey/errors.go

var ErrInvalidAPIKey = fault.NewWithStatus(
	"INVALID_PRODUCT_API_KEY",
	"Product API key is invalid",
	http.StatusUnauthorized,
)

func NewProductAPIKeyNameExistsError(name, productID string) *fault.Error {
	return fault.BadRequest("PRODUCT_API_KEY_NAME_DUPLICATE", "Product API key with this name already exists in the product").
		Metadata(map[string]any{
			errorDetailName:      name,
			errorDetailProductID: productID,
		})
}

func NewOrganizationAPIKeyNameExistsError(
	name, organizationID string,
) *fault.Error {
	return fault.BadRequest("ORGANIZATION_API_KEY_NAME_DUPLICATE", "Organization API key with this name already exists in the organization").
		Metadata(map[string]any{
			errorDetailName:   name,
			"organization_id": organizationID,
		})
}

func NewOrganizationAPIKeyInactiveOrExpiredError(apiKeyID string) *fault.Error {
	return fault.Forbidden("ORGANIZATION_API_KEY_INACTIVE_OR_EXPIRED", "Organization API key is inactive or expired").
		Metadata(map[string]any{
			errorDetailAPIKeyID: apiKeyID,
		})
}

func NewOrganizationAPIKeyExpiresAtInPastError() *fault.Error {
	return fault.BadRequest(
		"ORGANIZATION_API_KEY_EXPIRES_AT_IN_PAST",
		"Organization API key expiration date must be in the future",
	)
}

func NewProductAPIKeyInactiveError(apiKeyID string) *fault.Error {
	return fault.Forbidden("PRODUCT_API_KEY_INACTIVE", "Product API key is inactive").Metadata(map[string]any{
		errorDetailAPIKeyID: apiKeyID,
	})
}

func NewProductAPIKeyInsufficientPermissionsError(
	apiKeyID string, requiredScopes []string, currentScopes []string,
) *fault.Error {
	return fault.Forbidden("PRODUCT_API_KEY_INSUFFICIENT_PERMISSIONS", "Product API key does not have sufficient permissions").
		Metadata(map[string]any{
			errorDetailAPIKeyID: apiKeyID,
			"required_scopes":   requiredScopes,
			"current_scopes":    currentScopes,
		})
}

func NewProductAPIKeyPermissionsImmutableError(apiKeyID string) *fault.Error {
	return fault.BadRequest("PRODUCT_API_KEY_PERMISSIONS_IMMUTABLE", "Product API key permissions are immutable").
		Metadata(map[string]any{
			errorDetailAPIKeyID: apiKeyID,
		})
}

func NewPermissionsNotFoundError(
	productID string, permissionNames []string,
) *fault.Error {
	return fault.BadRequest("PERMISSIONS_NOT_FOUND", "Product permission does not exist").Metadata(map[string]any{
		errorDetailProductID: productID,
		"permission_names":   permissionNames,
	})
}

func NewOrganizationMembershipAlreadyExistsError(
	productUserID string,
	organizationID string,
) *fault.Error {
	return fault.NewWithStatus(
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
) *fault.Error {
	return fault.NewWithStatus(
		"ORGANIZATION_MEMBERSHIP_NOT_FOUND",
		fmt.Sprintf(
			"Organization membership not found for product user %s in organization %s",
			productUserID,
			organizationID,
		),
		http.StatusNotFound,
	)
}
