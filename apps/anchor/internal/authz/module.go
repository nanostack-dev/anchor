// Package authz keeps each Product's OpenFGA store, running inside Anchor on
// Anchor's Postgres, in step with Anchor's tables (ADR-0021; ADR-0022 E3 and
// E5). The tables stay the source of truth; the store mirrors them. Nothing
// reads the store for decisions yet.
//
// The pieces: engine runs OpenFGA; store gives each Product its store; fga is
// the generic OpenFGA surface; mirror is the sync engine; projection holds
// Anchor's projections, one per slice of the tables; model compiles a
// Product's catalog into its authorization model.
package authz

import (
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/mirror"
	"anchor/internal/authz/projection"
	"anchor/internal/authz/store"
	"anchor/internal/events"
)

func NewModule() fx.Option {
	return fx.Module(
		"authz",
		fx.Provide(
			engine.New,
			store.New,
			func(p engine.Provider) mirror.Engines { return p },
			func(s store.Stores) mirror.Stores { return s },
			mirror.NewPostgresRepository,
			mirror.NewPostgresLocker,
			projection.NewTables,
			fx.Annotate(projection.NewCatalog, fx.As(new(mirror.ModelSource))),
			mirror.AsProjection(projection.NewAssignments),
			mirror.AsProjection(projection.NewRoleGrants),
			mirror.AsProjection(projection.NewAPIKeyGrants),
			newMirror,
			events.AsTxObserver(newMarker),
		),
		fx.Invoke(func(m *mirror.Mirror, lc fx.Lifecycle) { m.Run(lc) }),
	)
}

// newMarker depends on the projections only: it runs inside every write that
// emits an event, so nothing it needs may emit events itself.
func newMarker(repo mirror.Repository, projections mirror.MarkerParams) (*mirror.Marker, error) {
	return mirror.NewMarker(repo, projections.Projections, projection.ChangesModel)
}

func newMirror(params mirror.Params, logger zerolog.Logger) (*mirror.Mirror, error) {
	return mirror.New(mirror.Deps{
		Engines: params.Engines, Stores: params.Stores, Repository: params.Repository, Locker: params.Locker,
		Models: params.Models, Projections: params.Projections, Logger: logger,
	})
}
