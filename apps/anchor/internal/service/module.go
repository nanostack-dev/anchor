package service

import (
	"anchor/internal/events"
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
			NewOrganizationAPIKeyService,
			NewOrganizationMembershipService,
			NewPermissionService,
			NewPlatformUserService,

			// Business services
			NewProductService,
			NewProductAPIKeyService,
			NewProductRoleService,
			NewOrganizationRoleService,
			NewResourcePermissionService,
			NewProductUserService,
			NewTenantService,

			// Integration services
			NewIntegrationQueue,
			NewIntegrationLock,
			NewIntegrationService,

			// Event registrations
			events.AsRegistration(OrganizationEventRegistration),
			events.AsRegistration(OrganizationAPIKeyEventRegistration),
			events.AsRegistration(ProductUserEventRegistration),
			events.AsRegistration(ProductRBACEventRegistration),
		),

		// Background workers
		fx.Invoke(RegisterProductPermissionStartupSync),
		fx.Invoke(RegisterAPIKeyEventWorker),
		fx.Invoke(RegisterIntegrationEventWorker),
	)
}
