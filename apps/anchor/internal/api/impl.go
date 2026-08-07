package api

import (
	emailsvc "anchor/internal/email/service"
	licensesvc "anchor/internal/license/service"
	"anchor/internal/service"
	"anchor/internal/service/config"

	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

var _ StrictServerInterface = (*AnchorAPI)(nil)

type AnchorAPI struct {
	TenantService                 service.TenantService
	AuthService                   service.AuthService
	PlatformInvitationService     service.InvitationService
	PlatformUserService           service.PlatformUserService
	PermissionService             service.PermissionService
	ProductService                service.ProductService
	ProductAPIKeyService          service.ProductAPIKeyService
	OrganizationAPIKeyService     service.OrganizationAPIKeyService
	ProductRoleService            service.ProductRoleService
	ProductUserService            service.ProductUserService
	ResourcePermissionService     service.ResourcePermissionService
	OrganizationService           service.OrganizationService
	WorkspaceService              service.WorkspaceService
	OrganizationMembershipService service.OrganizationMembershipService
	IntegrationService            service.IntegrationService
	EmailService                  emailsvc.EmailService
	LicenseSchemaService          licensesvc.LicenseSchemaService
	LicenseTemplateService        licensesvc.LicenseTemplateService
	OrganizationLicenseService    licensesvc.OrganizationLicenseService
	Queue                         *queue.Client
	CoreConfig                    *config.CoreConfig
	logger                        zerolog.Logger
}

type Params struct {
	fx.In
	TenantService                 service.TenantService
	AuthService                   service.AuthService
	PlatformInvitationService     service.InvitationService
	PlatformUserService           service.PlatformUserService
	PermissionService             service.PermissionService
	ProductService                service.ProductService
	ProductAPIKeyService          service.ProductAPIKeyService
	OrganizationAPIKeyService     service.OrganizationAPIKeyService
	ProductRoleService            service.ProductRoleService
	ProductUserService            service.ProductUserService
	ResourcePermissionService     service.ResourcePermissionService
	OrganizationService           service.OrganizationService
	WorkspaceService              service.WorkspaceService
	OrganizationMembershipService service.OrganizationMembershipService
	IntegrationService            service.IntegrationService
	EmailService                  emailsvc.EmailService
	LicenseSchemaService          licensesvc.LicenseSchemaService
	LicenseTemplateService        licensesvc.LicenseTemplateService
	OrganizationLicenseService    licensesvc.OrganizationLicenseService
	Queue                         *queue.Client
	CoreConfig                    *config.CoreConfig
	Logger                        zerolog.Logger
}

func NewAPI(params Params) *AnchorAPI {
	return &AnchorAPI{
		TenantService:                 params.TenantService,
		AuthService:                   params.AuthService,
		PlatformInvitationService:     params.PlatformInvitationService,
		PlatformUserService:           params.PlatformUserService,
		PermissionService:             params.PermissionService,
		ProductService:                params.ProductService,
		ProductAPIKeyService:          params.ProductAPIKeyService,
		OrganizationAPIKeyService:     params.OrganizationAPIKeyService,
		ProductRoleService:            params.ProductRoleService,
		ProductUserService:            params.ProductUserService,
		ResourcePermissionService:     params.ResourcePermissionService,
		OrganizationService:           params.OrganizationService,
		WorkspaceService:              params.WorkspaceService,
		OrganizationMembershipService: params.OrganizationMembershipService,
		IntegrationService:            params.IntegrationService,
		EmailService:                  params.EmailService,
		LicenseSchemaService:          params.LicenseSchemaService,
		LicenseTemplateService:        params.LicenseTemplateService,
		OrganizationLicenseService:    params.OrganizationLicenseService,
		Queue:                         params.Queue,
		CoreConfig:                    params.CoreConfig,
		logger:                        params.Logger.With().Str("component", "api_handler").Logger(),
	}
}
