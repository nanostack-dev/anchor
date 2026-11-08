package events

import (
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/nano"

	"anchor/internal/security/encryption"
	serviceconfig "anchor/internal/service/config"
)

// Module wires the product events: the catalog of every module's
// Registration, the emitter with every module's TxObserver, the endpoints and
// the delivery worker. The modules add members to the two collections.
var Module = nano.Module("events", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		NewCatalog,
		NewEndpointRepository,
		ProvideEmitter,
		ProvideEndpointService,
		NewWorker,
	),
	nano.Collect[Registration](),
	nano.Collect[TxObserver](),
)

func ProvideEndpointService(
	repo EndpointRepository,
	catalog Catalog,
	enc *encryption.Service,
	core *serviceconfig.CoreConfig,
	logger *slog.Logger,
) (EndpointService, error) {
	return NewEndpointService(repo, catalog, enc, core.IsProduction(), logger)
}
