package stripe

import (
	"anchor/internal/integration/provider"

	"go.uber.org/fx"
)

func NewModule() fx.Option {
	return fx.Module("stripe_provider",
		fx.Provide(NewProvider, provider.AsProviderResult(func(p *Provider) *Provider { return p })),
	)
}
