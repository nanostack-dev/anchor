package mapper

import (
	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module wires mapper dependencies.
var Module = nano.Module("mapper", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		NewProductMapper,
		NewOrganizationMapper,
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
		NewLicenseSchemaMapper,
		NewLicenseSchemaFieldMapper,
		NewLicenseTemplateMapper,
		NewOrganizationLicenseMapper,
		NewOrganizationLicenseChangeMapper,
		NewUsageObservationMapper,
	),
)
