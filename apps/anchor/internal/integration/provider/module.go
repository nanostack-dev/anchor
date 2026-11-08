package provider

import (
	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module provides the provider registry.
var Module = nano.Module("integration_providers", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		NewRegistry,
	),
	nano.Collect[Provider](),
)
