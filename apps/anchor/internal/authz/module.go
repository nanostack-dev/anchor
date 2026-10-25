// Package authz keeps each Product's OpenFGA store, running inside Anchor on
// Anchor's Postgres, in step with Anchor's tables (ADR-0021; ADR-0022 E3 and
// E5). The tables stay the source of truth; the store mirrors them, and every
// access decision reads it (ADR-0022 G4, PR 7).
package authz

import (
	"go.uber.org/fx"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/engine"
	"anchor/internal/authz/repository"
	"anchor/internal/authz/service"
	"anchor/internal/authz/store"
	"anchor/internal/events"
	resourceservice "anchor/internal/resource/service"
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
			fx.Annotate(service.NewChecker, fx.As(new(accessservice.PermissionChecker))),
			func(replayer *service.Replayer) resourceservice.WriteReplay { return replayer },
		),
		fx.Invoke(func(sweeper *service.Sweeper, lc fx.Lifecycle) { sweeper.Run(lc) }),
	)
}
