package mapper

import "go.uber.org/fx"

// NewModule wires mapper dependencies.
func NewModule() fx.Option {
	return fx.Module(
		"mapper",
		fx.Provide(
			NewProductMapper,
			NewOrganizationMapper,
			NewWorkspaceMapper,
			NewOrganizationAPIKeyMapper,
			NewTenantMapper,
			NewUserMapper,
			NewPlatformUserMapper,
			NewInvitationMapper,
			NewProductUserMapper,
			NewProductRoleMapper,
			NewProductPermissionMapper,
			NewProductResourcePermissionMapper,
			NewProductAPIKeyMapper,
			NewIntegrationInstanceMapper,
			NewIntegrationEventMapper,
			NewIntegrationAuditLogMapper,
			NewEmailTemplateMapper,
			NewEmailTemplateVersionMapper,
			NewEmailSendRecordMapper,
			NewPlanMapper,
			NewLicenseMapper,
		),
	)
}
