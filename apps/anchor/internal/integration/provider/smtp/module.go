package smtp

import (
	"anchor/internal/integration/provider"

	"go.uber.org/fx"
)

// NewModule registers the SMTP provider into the integration provider registry.
func NewModule() fx.Option {
	return fx.Module(
		"smtp_provider",
		fx.Provide(
			provider.AsProviderResult(NewProvider),
		),
	)
}
