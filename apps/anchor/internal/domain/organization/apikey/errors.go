package orgapikey

import "github.com/nanostack-dev/nanostack-framework/pkg/fault"

var (
	ErrOrganizationAPIKeyNotFound = fault.NotFound(
		"ORGANIZATION_API_KEY_NOT_FOUND",
		"This organization has no API key with that identifier",
	)
	ErrOrganizationAPIKeyCredentialNotFound = fault.NotFound(
		"ORGANIZATION_API_KEY_NOT_FOUND",
		"This product has no organization API key matching that credential",
	)
)
