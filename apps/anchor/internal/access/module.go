package access

import (
	"anchor/internal/access/service"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module wires access evaluation: the per-request decision a product asks
// Anchor for instead of rebuilding memberships, roles and resources itself.
var Module = nano.Module("access", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		service.NewEvaluator,
	),
)
