package events

import (
	"anchor/internal/security/encryption"
	serviceconfig "anchor/internal/service/config"

	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

func NewModule() fx.Option {
	return fx.Module(
		"events",
		fx.Provide(
			NewEndpointRepository,
			NewEmitter,
			provideEndpointService,
		),
		fx.Invoke(RegisterWorker),
	)
}

func provideEndpointService(
	repo EndpointRepository,
	enc *encryption.Service,
	core *serviceconfig.CoreConfig,
	logger zerolog.Logger,
) (EndpointService, error) {
	return NewEndpointService(repo, enc, core.IsProduction(), logger)
}
