package invitation

import (
	"anchor/internal/events"
	"anchor/internal/invitation/repository"
	"anchor/internal/invitation/service"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module wires the organization invitation subsystem: the invitation
// itself, its token and its lifecycle, and the accept that turns it into a
// membership.
var Module = nano.Module("organization_invitation", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		repository.NewRepository,
		service.NewOrganizationInvitationService,
	),
	nano.Collect[events.Registration](EventRegistration),
)
