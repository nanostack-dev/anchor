// Package authz keeps each Product's OpenFGA store, running inside Anchor on
// Anchor's Postgres, in step with Anchor's tables (ADR-0021; ADR-0022 E3 and
// E5). The tables stay the source of truth; the store holds the tuples they
// imply, and every access decision reads it (ADR-0022 G4, PR 7).
//
// The pieces: engine runs OpenFGA; store gives each Product its store; fga is
// the generic OpenFGA surface; fgasync is the sync engine; source holds
// Anchor's sources, one per slice of the tables; model compiles a
// Product's catalog into its authorization model; decision answers access
// decisions from the store.
package authz

import (
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/decision"
	"anchor/internal/authz/engine"
	"anchor/internal/authz/fgasync"
	"anchor/internal/authz/source"
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
			func(p engine.Provider) fgasync.Engines { return p },
			func(s store.Stores) fgasync.Stores { return s },
			fgasync.NewPostgresRepository,
			fgasync.NewPostgresLocker,
			source.NewTables,
			fx.Annotate(source.NewCatalog, fx.As(new(fgasync.ModelSource))),
			fgasync.AsSource(source.NewAssignments),
			fgasync.AsSource(source.NewRoleGrants),
			fgasync.AsSource(source.NewAPIKeyGrants),
			fgasync.AsSource(source.NewResourceParents),
			fgasync.AsSource(source.NewResourceAssignments),
			newSyncer,
			events.AsTxObserver(newMarker),
			newChecker,
			func(s *fgasync.Syncer) resourceservice.WriteReplay { return s },
		),
		fx.Invoke(func(m *fgasync.Syncer, lc fx.Lifecycle) { m.Run(lc) }),
	)
}

// newMarker depends on the sources only: it runs inside every write that
// emits an event, so nothing it needs may emit events itself.
func newMarker(repo fgasync.Repository, sources fgasync.MarkerParams) (*fgasync.Marker, error) {
	return fgasync.NewMarker(repo, sources.Sources, source.ChangesModel)
}

func newSyncer(params fgasync.Params, logger zerolog.Logger) (*fgasync.Syncer, error) {
	return fgasync.New(fgasync.Deps{
		Engines: params.Engines, Stores: params.Stores, Repository: params.Repository, Locker: params.Locker,
		Models: params.Models, Sources: params.Sources, Logger: logger,
	})
}

func newChecker(stores store.Stores, syncer *fgasync.Syncer) accessservice.PermissionChecker {
	return decision.NewChecker(stores, syncer)
}
