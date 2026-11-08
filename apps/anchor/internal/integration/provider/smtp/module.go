package smtp

import (
	"anchor/internal/integration/provider"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module registers the SMTP provider into the integration provider registry.
var Module = nano.Module("smtp_provider", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Collect[provider.Provider](NewProvider),
)
