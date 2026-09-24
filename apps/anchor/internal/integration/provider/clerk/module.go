package clerk

import (
	"anchor/internal/integration/provider"

	"go.uber.org/fx"
)

// NewModule registers the Clerk provider into the integration provider registry.
func NewModule() fx.Option {
	return fx.Module(
		"clerk_provider",
		fx.Provide(
			provider.AsProviderResult(NewProvider),
			fx.Annotate(
				WebhookEventRegistration,
				fx.ResultTags(`group:"integration_events"`),
			),
		),
	)
}
