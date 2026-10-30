package api

import (
	accesssvc "anchor/internal/access/service"
	emailsvc "anchor/internal/email/service"
	invitationsvc "anchor/internal/invitation/service"
	licensesvc "anchor/internal/license/service"
	resourceservice "anchor/internal/resource/service"
	rtservice "anchor/internal/resourcetype/service"
	assignmentservice "anchor/internal/roleassignment/service"
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
	OrganizationRoleService       service.OrganizationRoleService
	ProductUserService            service.ProductUserService
	ResourcePermissionService     service.ResourcePermissionService
	ResourceTypeService           rtservice.Service
	ResourceService               resourceservice.Service
	RoleAssignmentService         assignmentservice.Service
	OrganizationService           service.OrganizationService
	OrganizationMembershipService service.OrganizationMembershipService
	OrganizationInvitationService invitationsvc.OrganizationInvitationService
	IntegrationService            service.IntegrationService
	EmailService                  emailsvc.EmailService
	LicenseSchemaService          licensesvc.LicenseSchemaService
	LicenseTemplateService        licensesvc.LicenseTemplateService
	OrganizationLicenseService    licensesvc.OrganizationLicenseService
	LicenseHistoryService         licensesvc.LicenseHistoryService
	LicenseMigrationService       licensesvc.LicenseMigrationService
	UsageService                  licensesvc.UsageService
	UsageSeriesService            licensesvc.UsageSeriesService
	AccessEvaluator               accesssvc.Evaluator
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
	OrganizationRoleService       service.OrganizationRoleService
	ProductUserService            service.ProductUserService
	ResourcePermissionService     service.ResourcePermissionService
	ResourceTypeService           rtservice.Service
	ResourceService               resourceservice.Service
	RoleAssignmentService         assignmentservice.Service
	OrganizationService           service.OrganizationService
	OrganizationMembershipService service.OrganizationMembershipService
	OrganizationInvitationService invitationsvc.OrganizationInvitationService
	IntegrationService            service.IntegrationService
	EmailService                  emailsvc.EmailService
	LicenseSchemaService          licensesvc.LicenseSchemaService
	LicenseTemplateService        licensesvc.LicenseTemplateService
	OrganizationLicenseService    licensesvc.OrganizationLicenseService
	LicenseHistoryService         licensesvc.LicenseHistoryService
	LicenseMigrationService       licensesvc.LicenseMigrationService
	UsageService                  licensesvc.UsageService
	UsageSeriesService            licensesvc.UsageSeriesService
	AccessEvaluator               accesssvc.Evaluator
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
		OrganizationRoleService:       params.OrganizationRoleService,
		ProductUserService:            params.ProductUserService,
		ResourcePermissionService:     params.ResourcePermissionService,
		ResourceTypeService:           params.ResourceTypeService,
		ResourceService:               params.ResourceService,
		RoleAssignmentService:         params.RoleAssignmentService,
		OrganizationService:           params.OrganizationService,
		OrganizationMembershipService: params.OrganizationMembershipService,
		OrganizationInvitationService: params.OrganizationInvitationService,
		IntegrationService:            params.IntegrationService,
		EmailService:                  params.EmailService,
		LicenseSchemaService:          params.LicenseSchemaService,
		LicenseTemplateService:        params.LicenseTemplateService,
		OrganizationLicenseService:    params.OrganizationLicenseService,
		LicenseHistoryService:         params.LicenseHistoryService,
		LicenseMigrationService:       params.LicenseMigrationService,
		UsageService:                  params.UsageService,
		UsageSeriesService:            params.UsageSeriesService,
		AccessEvaluator:               params.AccessEvaluator,
		Queue:                         params.Queue,
		CoreConfig:                    params.CoreConfig,
		logger:                        params.Logger.With().Str("component", "api_handler").Logger(),
	}
}
