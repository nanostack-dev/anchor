package organization

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

var (
	ErrOrganizationNotFound = fault.NotFound(
		"ORGANIZATION_NOT_FOUND",
		"This product has no organization with that identifier",
	)
	ErrMembershipNotFound = fault.NotFound(
		"ORGANIZATION_MEMBERSHIP_NOT_FOUND",
		"This organization has no membership for that product user",
	)
)
