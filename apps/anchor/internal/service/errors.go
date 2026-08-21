package service

import (
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

const (
	errorDetailName      = fieldNameKey
	errorDetailProductID = fieldProductIDKey
	errorDetailAPIKeyID  = fieldAPIKeyIDKey
)

// From auth/errors.go

var (
	// ErrInvalidCredentials for a login whose email or password does not
	// authenticate. The password is wrong, the email matches no user, or the
	// user has no membership in the single tenant Anchor serves today — every
	// branch is the same credential failure, so the client cannot tell which
	// one occurred.
	ErrInvalidCredentials = fault.Unauthorized(
		"EMAIL_OR_PASSWORD_INCORRECT",
		"The email or the password is incorrect.",
	)

	// ErrUserAlreadyExists when trying to register an existing email. A
	// uniqueness collision on the email field: a different email in a later
	// request succeeds, so this is a conflict, not a bad request.
	ErrUserAlreadyExists = fault.Conflict(
		"USER_ALREADY_EXISTS",
		"A user with this email already exists.",
	)

	// ErrInvitationCodeNotProvided when a registration attempt lacks an
	// invitation code. The code is a body field, not a credential: it names an
	// invitation record, it does not authenticate the requester.
	ErrInvitationCodeNotProvided = fault.BadRequest(
		"INVITATION_CODE_NOT_PROVIDED",
		"The invitation code is required.",
	)

	// ErrInvitationCodeIsInvalid when the code names no invitation. Same
	// addressing as ErrInvitationCodeNotProvided: a body-named entity that
	// does not resolve.
	ErrInvitationCodeIsInvalid = fault.BadRequest(
		"INVITATION_CODE_IS_INVALID",
		"The invitation code is invalid.",
	)

	// ErrUserNotFound when a refresh token's claims name a user that no
	// longer exists. The caller never named this user — the identifier comes
	// from the token itself — so this is the token no longer authenticating
	// anyone, the same 401 family as ErrTokenRefreshFailed.
	ErrUserNotFound = fault.Unauthorized(
		"USER_NOT_FOUND",
		"The user for this refresh token no longer exists.",
	)

	// ErrTokenRefreshFailed for issues refreshing JWTs.
	ErrTokenRefreshFailed = fault.Unauthorized(
		"TOKEN_REFRESH_FAILED",
		"The refresh token is invalid or expired.",
	)

	// ErrProductAlreadyExists is a uniqueness collision on the product name,
	// the same shape as ErrUserAlreadyExists: a later request with a
	// different name succeeds.
	ErrProductAlreadyExists = fault.Conflict(
		"PRODUCT_ALREADY_EXISTS", "A product with this name already exists in your tenant.",
	)

	ErrOwnerRoleNotAllowed = fault.BadRequest(
		"INVITATION_OWNER_ROLE_NOT_ALLOWED",
		"An invitation with the OWNER role is not allowed.",
	)

	// ErrInvitationAlreadyExists when the tenant already has an invitation for
	// the address. Returned both by CreateInvitation's pre-insert check and by
	// the race loser whose INSERT trips the unique index. A uniqueness
	// collision: inviting a different address succeeds.
	ErrInvitationAlreadyExists = fault.Conflict(
		"INVITATION_ALREADY_EXISTS",
		"This email address already has an invitation. "+
			"Check if the person is already a member, or invite a different email address.",
	)
)

// From resource_permission/errors.go

func NewResourcePermissionInUseError(resourcePermissionID string, roleCount int) error {
	return fault.Conflict("RESOURCE_PERMISSION_IN_USE", fmt.Sprintf(
		"You cannot delete resource permission %s. It is assigned to %d role(s).",
		resourcePermissionID, roleCount,
	)).Metadata(map[string]any{
		"resource_permission_id": resourcePermissionID,
		"role_count":             roleCount,
	})
}

func NewResourcePermissionAlreadyExistsError(name string) error {
	return fault.Conflict("RESOURCE_PERMISSION_ALREADY_EXISTS", fmt.Sprintf("A resource permission with the name '%s' already exists.", name)).
		Metadata(map[string]any{
			errorDetailName: name,
		})
}

// From role/errors.go

// NewRoleNotFoundError reports a role the caller named that does not exist.
//
// Every call site names the role in the URL path
// (/v1/products/{product_id}/roles/{role_id}/...), so 404 is correct here.
// A role_id read from a request body answers 400 with a distinct code through
// newBodyRoleNotFoundError in organization_membership_service.go.
func NewRoleNotFoundError(roleID string) *fault.Error {
	return fault.NotFound(
		"ROLE_NOT_FOUND",
		fmt.Sprintf("Product role %s does not exist.", roleID),
	)
}

// NewProductUserNotFoundError reports a product user named in the URL path
// that does not exist. A product_user_id read from a request body answers 400
// with a distinct code through newBodyProductUserNotFoundError in
// organization_membership_service.go.
func NewProductUserNotFoundError(productUserID string) *fault.Error {
	return fault.NotFound(
		"PRODUCT_USER_NOT_FOUND",
		fmt.Sprintf("Product user %s does not exist.", productUserID),
	)
}

// NewRoleWithAlreadyExistingNameError is a uniqueness collision on the role
// name: a different name in a later request succeeds.
func NewRoleWithAlreadyExistingNameError(roleName, productID string) *fault.Error {
	return fault.Conflict("ROLE_NAME_DUPLICATE", "A product role with this name already exists in the product.").
		Metadata(map[string]any{
			"role_name":          roleName,
			errorDetailProductID: productID,
		})
}

// NewPermissionAlreadyAssignedError is a uniqueness collision on the
// role/permission pair. Not currently returned by any call site.
func NewPermissionAlreadyAssignedError(roleID, permissionName string) *fault.Error {
	return fault.Conflict("PERMISSION_ALREADY_ASSIGNED", "The permission is already assigned to the role.").
		Metadata(map[string]any{
			"role_id":         roleID,
			"permission_name": permissionName,
		})
}

// NewRoleInUseError blocks a delete because current state (users still hold
// the role) refuses it. Removing those assignments lets a later delete
// succeed, so this is a conflict, not a bad request.
func NewRoleInUseError(roleID string) *fault.Error {
	return fault.Conflict(
		"ROLE_IN_USE",
		fmt.Sprintf("You cannot delete product role %s. It is assigned to users.", roleID),
	)
}

// From product/apikey/errors.go

var ErrInvalidAPIKey = fault.Unauthorized(
	"INVALID_PRODUCT_API_KEY",
	"The product API key is invalid.",
)

// NewProductAPIKeyNameExistsError is a uniqueness collision on the key name.
func NewProductAPIKeyNameExistsError(name, productID string) *fault.Error {
	return fault.Conflict("PRODUCT_API_KEY_NAME_DUPLICATE", "A product API key with this name already exists in the product.").
		Metadata(map[string]any{
			errorDetailName:      name,
			errorDetailProductID: productID,
		})
}

// NewOrganizationAPIKeyNameExistsError is a uniqueness collision on the key name.
func NewOrganizationAPIKeyNameExistsError(
	name, organizationID string,
) *fault.Error {
	return fault.Conflict("ORGANIZATION_API_KEY_NAME_DUPLICATE", "An organization API key with this name already exists in the organization.").
		Metadata(map[string]any{
			errorDetailName:   name,
			"organization_id": organizationID,
		})
}

// NewOrganizationAPIKeyInactiveOrExpiredError blocks setting an expired key
// back to active. It is not a permission problem: the caller is authorized to
// update the key, but its current state refuses this particular update.
// Extending the expiration date first lets a later activation succeed, so
// this is a conflict.
func NewOrganizationAPIKeyInactiveOrExpiredError(apiKeyID string) *fault.Error {
	return fault.Conflict("ORGANIZATION_API_KEY_INACTIVE_OR_EXPIRED",
		"You cannot activate an expired organization API key. Extend the expiration date first.").
		Metadata(map[string]any{
			errorDetailAPIKeyID: apiKeyID,
		})
}

func NewOrganizationAPIKeyExpiresAtInPastError() *fault.Error {
	return fault.BadRequest(
		"ORGANIZATION_API_KEY_EXPIRES_AT_IN_PAST",
		"The organization API key expiration date must be in the future.",
	)
}

// NewProductAPIKeyInactiveError rejects an API key credential that is
// present, well-formed, and no longer active. An inactive credential does not
// authenticate, so this is a 401, not a 403.
func NewProductAPIKeyInactiveError(apiKeyID string) *fault.Error {
	return fault.Unauthorized("PRODUCT_API_KEY_INACTIVE", "The product API key is inactive.").Metadata(map[string]any{
		errorDetailAPIKeyID: apiKeyID,
	})
}

// NewProductAPIKeyInsufficientPermissionsError reports that the key
// authenticated, and the principal it identifies does not hold the required
// scopes.
func NewProductAPIKeyInsufficientPermissionsError(
	apiKeyID string, requiredScopes []string, currentScopes []string,
) *fault.Error {
	return fault.Forbidden("PRODUCT_API_KEY_INSUFFICIENT_PERMISSIONS", "The product API key does not have sufficient permissions.").
		Metadata(map[string]any{
			errorDetailAPIKeyID: apiKeyID,
			"required_scopes":   requiredScopes,
			"current_scopes":    currentScopes,
		})
}

func NewProductAPIKeyPermissionsImmutableError(apiKeyID string) *fault.Error {
	return fault.BadRequest("PRODUCT_API_KEY_PERMISSIONS_IMMUTABLE", "You cannot change product API key permissions after creation.").
		Metadata(map[string]any{
			errorDetailAPIKeyID: apiKeyID,
		})
}

func NewPermissionsNotFoundError(
	productID string, permissionNames []string,
) *fault.Error {
	return fault.BadRequest("PERMISSIONS_NOT_FOUND", "The product permission does not exist.").Metadata(map[string]any{
		errorDetailProductID: productID,
		"permission_names":   permissionNames,
	})
}

func NewOrganizationMembershipAlreadyExistsError(
	productUserID string,
	organizationID string,
) *fault.Error {
	return fault.Conflict(
		"ORGANIZATION_MEMBERSHIP_ALREADY_EXISTS",
		fmt.Sprintf(
			"Organization membership already exists for product user %s in organization %s.",
			productUserID,
			organizationID,
		),
	)
}

func NewOrganizationMembershipNotFoundError(
	productUserID string,
	organizationID string,
) *fault.Error {
	return fault.NotFound(
		"ORGANIZATION_MEMBERSHIP_NOT_FOUND",
		fmt.Sprintf(
			"Organization membership not found for product user %s in organization %s.",
			productUserID,
			organizationID,
		),
	)
}

func NewOrganizationLicenseTemplateNotFoundError(templateID string) *fault.Error {
	return fault.BadRequest(
		"ORGANIZATION_LICENSE_TEMPLATE_NOT_FOUND",
		"This product has no license template with that identifier",
	).Metadata(map[string]any{
		"template_id": templateID,
	})
}

func NewOrganizationMetadataTooManyKeysError(keyCount, maxKeys int) *fault.Error {
	return fault.BadRequest(
		"ORGANIZATION_METADATA_TOO_MANY_KEYS",
		fmt.Sprintf("Organization metadata accepts at most %d keys, got %d", maxKeys, keyCount),
	).Metadata(map[string]any{
		"key_count": keyCount,
		"max_keys":  maxKeys,
	})
}

func NewOrganizationMetadataInvalidKeyError(key string, maxKeyLength int) *fault.Error {
	return fault.BadRequest(
		"ORGANIZATION_METADATA_INVALID_KEY",
		fmt.Sprintf(
			"Organization metadata keys must be non-blank and at most %d characters",
			maxKeyLength,
		),
	).Metadata(map[string]any{
		fieldMetadataKeyKey: key,
	})
}

func NewOrganizationMetadataInvalidValueError(key string) *fault.Error {
	return fault.BadRequest(
		"ORGANIZATION_METADATA_INVALID_VALUE",
		"Organization metadata values must be a string, number, or boolean",
	).Metadata(map[string]any{
		fieldMetadataKeyKey: key,
	})
}

func NewOrganizationMetadataValueTooLongError(key string, maxValueLength int) *fault.Error {
	return fault.BadRequest(
		"ORGANIZATION_METADATA_VALUE_TOO_LONG",
		fmt.Sprintf(
			"Organization metadata string values must be at most %d characters",
			maxValueLength,
		),
	).Metadata(map[string]any{
		fieldMetadataKeyKey: key,
	})
}
