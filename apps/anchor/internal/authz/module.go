// Package authz decides access with OpenFGA, the open-source implementation of
// Google's Zanzibar, running inside Anchor on Anchor's Postgres.
//
// Postgres stays the source of truth for organizations, members, workspaces,
// roles, permissions and API keys. Each product gets an OpenFGA store whose
// model is built from the product's roles and permissions, and whose tuples
// mirror its organizations. See docs/technical/authorization-openfga.md.
package authz

import (
	"go.uber.org/fx"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/repository"
	"anchor/internal/authz/service"
	"anchor/internal/events"
)

func NewModule() fx.Option {
	return fx.Module(
		"authz",
		fx.Provide(
			engine.New,
			repository.New,
			service.NewSyncer,
			service.NewAuthorizer,
			service.NewSweeper,
			events.AsTxObserver(service.NewMarker),
		),
		fx.Invoke(func(sweeper *service.Sweeper, lc fx.Lifecycle) { sweeper.Run(lc) }),
	)
}
