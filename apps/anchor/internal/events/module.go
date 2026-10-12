package events

import (
	"anchor/internal/security/encryption"
	serviceconfig "anchor/internal/service/config"

	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"
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

type emitterParams struct {
	fx.In
	Queue     *queue.Client
	Catalog   Catalog
	Listeners []Listener `group:"product_event_listeners"`
}

func provideEmitter(p emitterParams) Emitter {
	return NewEmitter(p.Queue, p.Catalog, p.Listeners...)
}

func provideEndpointService(
	repo EndpointRepository,
	catalog Catalog,
	enc *encryption.Service,
	core *serviceconfig.CoreConfig,
	logger zerolog.Logger,
) (EndpointService, error) {
	return NewEndpointService(repo, catalog, enc, core.IsProduction(), logger)
}
