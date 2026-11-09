package api

import (
	"log/slog"

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
)

var _ StrictServerInterface = (*AnchorAPI)(nil)

// AnchorAPI holds the services the operation handlers call.
type AnchorAPI struct { //nolint:dupl // NewAPI takes each field as a parameter, as the generated wiring calls it
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
	logger                        *slog.Logger
}

// NewAPI builds the handlers of every operation. nanostack gen wire passes each
// dependency as a parameter, so the list mirrors the AnchorAPI fields.
//
//nolint:dupl // see AnchorAPI
func NewAPI(
	tenantService service.TenantService,
	authService service.AuthService,
	platformInvitationService service.InvitationService,
	platformUserService service.PlatformUserService,
	permissionService service.PermissionService,
	productService service.ProductService,
	productAPIKeyService service.ProductAPIKeyService,
	organizationAPIKeyService service.OrganizationAPIKeyService,
	productRoleService service.ProductRoleService,
	organizationRoleService service.OrganizationRoleService,
	productUserService service.ProductUserService,
	resourcePermissionService service.ResourcePermissionService,
	resourceTypeService rtservice.Service,
	resourceService resourceservice.Service,
	roleAssignmentService assignmentservice.Service,
	organizationService service.OrganizationService,
	organizationMembershipService service.OrganizationMembershipService,
	organizationInvitationService invitationsvc.OrganizationInvitationService,
	integrationService service.IntegrationService,
	emailService emailsvc.EmailService,
	licenseSchemaService licensesvc.LicenseSchemaService,
	licenseTemplateService licensesvc.LicenseTemplateService,
	organizationLicenseService licensesvc.OrganizationLicenseService,
	licenseHistoryService licensesvc.LicenseHistoryService,
	licenseMigrationService licensesvc.LicenseMigrationService,
	usageService licensesvc.UsageService,
	usageSeriesService licensesvc.UsageSeriesService,
	accessEvaluator accesssvc.Evaluator,
	queueClient *queue.Client,
	coreConfig *config.CoreConfig,
	logger *slog.Logger,
) *AnchorAPI {
	return &AnchorAPI{
		TenantService:                 tenantService,
		AuthService:                   authService,
		PlatformInvitationService:     platformInvitationService,
		PlatformUserService:           platformUserService,
		PermissionService:             permissionService,
		ProductService:                productService,
		ProductAPIKeyService:          productAPIKeyService,
		OrganizationAPIKeyService:     organizationAPIKeyService,
		ProductRoleService:            productRoleService,
		OrganizationRoleService:       organizationRoleService,
		ProductUserService:            productUserService,
		ResourcePermissionService:     resourcePermissionService,
		ResourceTypeService:           resourceTypeService,
		ResourceService:               resourceService,
		RoleAssignmentService:         roleAssignmentService,
		OrganizationService:           organizationService,
		OrganizationMembershipService: organizationMembershipService,
		OrganizationInvitationService: organizationInvitationService,
		IntegrationService:            integrationService,
		EmailService:                  emailService,
		LicenseSchemaService:          licenseSchemaService,
		LicenseTemplateService:        licenseTemplateService,
		OrganizationLicenseService:    organizationLicenseService,
		LicenseHistoryService:         licenseHistoryService,
		LicenseMigrationService:       licenseMigrationService,
		UsageService:                  usageService,
		UsageSeriesService:            usageSeriesService,
		AccessEvaluator:               accessEvaluator,
		Queue:                         queueClient,
		CoreConfig:                    coreConfig,
		logger:                        logger.With(slog.String("component", "api_handler")),
	}
}
