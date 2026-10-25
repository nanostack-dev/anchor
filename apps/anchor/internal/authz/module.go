// Package authz keeps each Product's OpenFGA store, running inside Anchor on
// Anchor's Postgres, in step with Anchor's tables (ADR-0021; ADR-0022 E3 and
// E5). The tables stay the source of truth; the store mirrors them. Nothing
// reads the store for decisions yet.
package authz

import (
	"go.uber.org/fx"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/repository"
	"anchor/internal/authz/service"
	"anchor/internal/authz/store"
	"anchor/internal/events"
)

func NewModule() fx.Option {
	return fx.Module(
		"authz",
		fx.Provide(
			engine.New,
			store.New,
			repository.New,
			service.NewSyncer,
			service.NewSweeper,
			service.NewReplayer,
			events.AsTxObserver(service.NewMarker),
		),
		fx.Invoke(func(sweeper *service.Sweeper, lc fx.Lifecycle) { sweeper.Run(lc) }),
	)
}
