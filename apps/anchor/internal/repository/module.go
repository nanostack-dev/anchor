package repository

import (
	"go.uber.org/fx"
)

func NewModule() fx.Option {
	return fx.Module(
		"repository",
		fx.Provide(
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
}
