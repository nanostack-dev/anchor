package service

import (
	"anchor/internal/service/config"

	"go.uber.org/fx"
)

func NewModule() fx.Option {
	return fx.Module(
		"service",
		config.NewModule(),
		fx.Provide(
			NewOrganizationAPIKeyEventService,
			NewJWTHelper,
			NewAuthService,
			NewInvitationService,
			NewOrganizationService,
			NewWorkspaceService,
			NewOrganizationAPIKeyService,
			NewOrganizationMembershipService,
			NewPermissionService,
			NewPlatformUserService,

			// Cache services
			NewProductCacheService,
			NewProductAPIKeyCacheService,

			// Business services
			NewProductService,
			NewProductAPIKeyService,
			NewProductRoleService,
			NewResourcePermissionService,
			NewProductUserService,
			NewTenantService,

			// Integration services
			NewIntegrationQueue,
			NewIntegrationLock,
			NewIntegrationService,

			// License services
			NewPlanService,
			NewLicenseService,

			// Outbound webhook services
			NewWebhookHTTPClient,
			NewWebhookEmitter,
			NewWebhookEndpointService,
			NewWebhookFanoutService,
			NewWebhookDeliveryService,
		),

		// Background workers
		fx.Invoke(RegisterProductPermissionStartupSync),
		fx.Invoke(RegisterAPIKeyEventWorker),
		fx.Invoke(RegisterIntegrationEventWorker),
		fx.Invoke(RegisterWebhookWorkers),
	)
}
