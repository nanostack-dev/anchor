package provider

import "go.uber.org/fx"

// NewModule provides the provider registry.
func NewModule() fx.Option {
	return fx.Module(
		"integration_providers",
		fx.Provide(
			NewRegistry,
		),
	)
}
