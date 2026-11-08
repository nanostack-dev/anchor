package service

import (
	"github.com/nanostack-dev/nanostack-framework/core/nano"

	"anchor/internal/events"
	"anchor/internal/service/config"
)

// Module wires the business services, the integration queue and its locks,
// and the background work: the startup permission sync, the API key event
// and integration workers and the queue retention.
var Module = nano.Module("service", //nolint:gochecknoglobals // A nano module is a package-level value.
	config.Module,
	nano.Providers(
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
		NewProductCache,
		NewProductService,
		NewProductAPIKeyCache,
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
		// Background work
		NewProductPermissionStartupSync,
		NewAPIKeyEventWorker,
		NewIntegrationEventWorker,
		NewQueueRetention,
	),
	nano.Collect[events.Registration](
		OrganizationEventRegistration,
		OrganizationAPIKeyEventRegistration,
		ProductUserEventRegistration,
		ProductRBACEventRegistration,
	),
)
