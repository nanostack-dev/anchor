package clerk

import (
	"anchor/internal/events"
	"anchor/internal/integration/provider"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module registers the Clerk provider into the integration provider registry.
var Module = nano.Module("clerk_provider", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Collect[provider.Provider](NewProvider),
	nano.Collect[events.Registration](WebhookEventRegistration),
)
