package authz

import (
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/nano"

	accessservice "anchor/internal/access/service"
	"anchor/internal/applog"
	"anchor/internal/authz/decision"
	"anchor/internal/authz/engine"
	"anchor/internal/authz/fgasync"
	"anchor/internal/authz/source"
	"anchor/internal/authz/store"
	"anchor/internal/events"
)

// Module wires the authorization store: the in-process OpenFGA engine, the
// per-Product stores, the sources that feed them, the syncer that keeps them
// current, and the checker that answers decisions.
var Module = nano.Module("authz", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		engine.New,
		store.New,
		fgasync.NewPostgresRepository,
		fgasync.NewPostgresLocker,
		source.NewTables,
		source.NewCatalog,
		NewSyncer,
		NewChecker,
	),
	nano.Bind[fgasync.Engines, engine.Provider](),
	nano.Bind[fgasync.Stores, store.Stores](),
	nano.Bind[fgasync.ModelSource, source.Catalog](),
	nano.Collect[fgasync.Source](
		source.NewAssignments,
		source.NewRoleGrants,
		source.NewAPIKeyGrants,
		source.NewResourceParents,
		source.NewResourceAssignments,
	),
	nano.Collect[events.TxObserver](NewMarker, source.NewModelSizeGuard),
)

// NewMarker depends on the sources only: it runs inside every write that
// emits an event, so nothing it needs may emit events itself.
func NewMarker(repo fgasync.Repository, sources []fgasync.Source) (*fgasync.Marker, error) {
	return fgasync.NewMarker(repo, sources, source.ChangesModel)
}

func NewSyncer(
	engines fgasync.Engines, stores fgasync.Stores, repo fgasync.Repository, locker fgasync.Locker,
	models fgasync.ModelSource, sources []fgasync.Source, logger *slog.Logger,
) (*fgasync.Syncer, error) {
	return fgasync.New(fgasync.Deps{
		Engines: engines, Stores: stores, Repository: repo, Locker: locker,
		Models: models, Sources: sources, Logger: logger,
	})
}

func NewChecker(stores store.Stores, syncer *fgasync.Syncer, logger applog.Scoped) accessservice.PermissionChecker {
	return decision.NewChecker(stores, syncer, logger)
}
