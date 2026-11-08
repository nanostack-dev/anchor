package events

import (
	"log/slog"

	"anchor/internal/security/encryption"
	serviceconfig "anchor/internal/service/config"

	"go.uber.org/fx"
)

func NewModule() fx.Option {
	return fx.Module(
		"events",
		fx.Provide(
			NewCatalog,
			NewEndpointRepository,
			provideEmitter,
			provideEndpointService,
		),
		fx.Invoke(RegisterWorker),
	)
}

func provideEndpointService(
	repo EndpointRepository,
	catalog Catalog,
	enc *encryption.Service,
	core *serviceconfig.CoreConfig,
	logger *slog.Logger,
) (EndpointService, error) {
	return NewEndpointService(repo, catalog, enc, core.IsProduction(), logger)
}
