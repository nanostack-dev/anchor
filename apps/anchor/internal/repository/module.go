package repository

import (
	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

var Module = nano.Module("repository", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		NewPlatformInvitationRepository,
		NewOrganizationRepository,
		NewOrganizationAPIKeyRepository,
		NewProductRepository,
		NewProductPermissionRepository,
		NewProductResourcePermissionRepository,
		NewProductUserRepository,
		NewTenantRepository,
		NewUserRepository,
		NewPlatformTenantUserRepository,
		NewProductRoleRepository,
		NewProductAPIKeyRepository,
		NewIntegrationInstanceRepository,
		NewIntegrationEventRepository,
		NewIntegrationAuditLogRepository,
		NewOrganizationMembershipRepository,
	),
)
