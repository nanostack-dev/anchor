package invitation

import (
	"anchor/internal/events"
	"anchor/internal/invitation/repository"
	"anchor/internal/invitation/service"

	"go.uber.org/fx"
)

// NewModule wires the organization invitation subsystem: the invitation
// itself, its token and its lifecycle, and the accept that turns it into a
// membership.
func NewModule() fx.Option {
	return fx.Module(
		"organization_invitation",
		fx.Provide(
			repository.NewRepository,
			service.NewOrganizationInvitationService,
			events.AsRegistration(EventRegistration),
		),
	)
}
