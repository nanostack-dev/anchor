// Package store gives each Product its OpenFGA store, the relationship store
// that mirrors its access data (ADR-0022 E5). Anchor records which store
// belongs to which Product.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"

	"anchor/internal/applog"
	"anchor/internal/authz/engine"
	"anchor/internal/authz/fga"
	authzmodel "anchor/internal/authz/model"
	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
)

var (
	// ErrProductNotFound is returned for a store requested for no existing Product.
	ErrProductNotFound = errors.New("product not found")
	// ErrInTransaction is returned when StoreID runs inside a transaction: the
	// OpenFGA store it may create commits on its own, so a rollback of the
	// caller's transaction would leave it behind unrecorded.
	ErrInTransaction = errors.New("authz store lookup must not run inside a transaction")
)

const productForeignKey = "authz_stores_product_id_fkey"

type Stores interface {
	// StoreID returns the Product's OpenFGA store, creating it on first use.
	// Two replicas creating one at once keep one store and delete the other.
	// It must run outside any transaction.
	StoreID(ctx context.Context, productID string) (string, error)
	// Open returns the Product's store at the model it follows, or none while
	// either is not recorded yet. It never creates a store. Until the engine is
	// up it answers engine.ErrNotReady.
	Open(ctx context.Context, productID string) (functional.Option[Opened], error)
}

// Opened is a Product's store at the model it follows.
type Opened struct {
	Store     fga.Store
	Relations fga.Relations
	// Rebuilt reports whether a full rebuild filled the store. Until one does,
	// it may hold only the keys written since, and would deny the others.
	Rebuilt bool
	// ModelCurrent reports whether Store.ModelID() came from this release's
	// authz_models row (as opposed to a fallback model from another release or
	// the legacy authz_stores pointer while this release's row is not yet written).
	ModelCurrent bool
	// refused samples the log of a contextual tuple the model refuses.
	refused *applog.BurstSampler
	// logger writes the refusal with the request scope of its context. Nil
	// writes nothing.
	logger *slog.Logger
}

// Holds answers, for each tuple in order, whether its user holds its relation
// on its object, with the contextual tuples holding for these checks only. A
// relation the model doesn't define is not held, and is not asked; neither is
// anything when a contextual tuple is one the model wouldn't accept, which
// OpenFGA answers with an error rather than a denial. That case is logged:
// the catalog allows a parent the model doesn't yet. Any check OpenFGA fails
// on fails the call.
func (o Opened) Holds(ctx context.Context, tuples []fga.Tuple, contextual ...fga.Tuple) ([]bool, error) {
	held := make([]bool, len(tuples))
	for _, t := range contextual {
		if !o.Relations.Accepts(t) {
			if o.logger != nil && o.refused.Sample() {
				o.logger.WarnContext(ctx, "authz.contextual_tuple_refused",
					slog.String("event", "authz.contextual_tuple_refused"),
					slog.String("model_id", o.Store.ModelID()),
					slog.String("tuple", t.String()),
				)
			}
			return held, nil
		}
	}
	var (
		asked   []fga.Tuple
		indexes []int
	)
	for i, t := range tuples {
		if o.Relations.Defines(t) {
			asked = append(asked, t)
			indexes = append(indexes, i)
		}
	}
	answers, err := o.Store.CheckAll(ctx, asked, contextual...)
	if err != nil {
		return nil, err
	}
	for n, i := range indexes {
		held[i] = answers[n]
	}
	return held, nil
}

// refusedLogBurst caps the refused contextual tuples logged a minute.
const refusedLogBurst = 20

type stores struct {
	db       *sql.DB
	provider engine.Provider
	version  string
	logger   *slog.Logger
	scoped   *slog.Logger
	refused  *applog.BurstSampler
	// relations holds, per model ID, what the model defines: a model never
	// changes once written.
	relations sync.Map
}

func New(db *sql.DB, provider engine.Provider, logger *slog.Logger, scoped applog.Scoped) Stores {
	return NewWithVersion(db, provider, logger, scoped, authzmodel.Version)
}

// NewWithVersion opens stores for a specific compiled model version. Production
// uses New (model.Version); tests use this to simulate two releases running
// side by side during a rolling deploy.
func NewWithVersion(
	db *sql.DB, provider engine.Provider, logger *slog.Logger, scoped applog.Scoped, version string,
) Stores {
	if version == "" {
		version = authzmodel.Version
	}
	return &stores{
		db: db, provider: provider, version: version,
		logger:  logger.With(slog.String("component", "authz_store")),
		scoped:  scoped.Logger,
		refused: &applog.BurstSampler{Burst: refusedLogBurst, Period: time.Minute},
	}
}

func (s *stores) StoreID(ctx context.Context, productID string) (string, error) {
	if transactor.CurrentTx(ctx) != nil {
		return "", ErrInTransaction
	}
	recorded, err := s.find(ctx, productID)
	if err != nil || recorded != "" {
		return recorded, err
	}
	openfga, err := s.provider.Get()
	if err != nil {
		return "", err
	}
	created, err := openfga.CreateStore(ctx, &openfgav1.CreateStoreRequest{Name: productID})
	if err != nil {
		return "", fmt.Errorf("create openfga store: %w", err)
	}
	inserted, err := transactor.Query[[]model.AuthzStores](ctx, s.db,
		table.AuthzStores.INSERT(table.AuthzStores.ProductID, table.AuthzStores.StoreID).
			VALUES(productID, created.GetId()).
			ON_CONFLICT(table.AuthzStores.ProductID).DO_NOTHING().
			RETURNING(table.AuthzStores.StoreID),
	).OnForeignKey(ErrProductNotFound, productForeignKey).Value()
	if errors.Is(err, ErrProductNotFound) {
		s.discard(ctx, openfga, created.GetId())
		return "", err
	}
	if err == nil && len(inserted) > 0 {
		return inserted[0].StoreID, nil
	}
	// The insert lost a race, or failed in a way that may still have committed
	// it, such as a cancelled request: whatever is recorded now wins, and this
	// store is deleted only if it is not the one recorded.
	winner, findErr := s.find(context.WithoutCancel(ctx), productID)
	if findErr != nil {
		return "", errors.Join(err, findErr)
	}
	if winner != created.GetId() {
		s.discard(ctx, openfga, created.GetId())
	}
	if winner == "" {
		return "", err
	}
	return winner, nil
}

type openedRow struct {
	StoreID       string
	ReconciledAt  *time.Time
	LegacyModelID *string
	ModelID       *string
}

func (s *stores) Open(ctx context.Context, productID string) (functional.Option[Opened], error) {
	openfga, err := s.provider.Get()
	if err != nil {
		return functional.None[Opened](), err
	}
	row, found, err := s.findStoreAndModel(ctx, productID)
	if err != nil || !found {
		return functional.None[Opened](), err
	}
	modelID, modelCurrent, err := s.resolveModelID(ctx, productID, row)
	if err != nil || modelID == "" {
		return functional.None[Opened](), err
	}
	opened := Opened{
		Store:        fga.NewStore(openfga, row.StoreID, modelID),
		Rebuilt:      row.ReconciledAt != nil,
		ModelCurrent: modelCurrent,
		refused:      s.refused,
		logger:       s.scoped,
	}
	if cached, hit := s.relations.Load(opened.Store.ModelID()); hit {
		if relations, isRelations := cached.(fga.Relations); isRelations {
			opened.Relations = relations
			return functional.Some(opened), nil
		}
	}
	if opened.Relations, err = opened.Store.Relations(ctx); err != nil {
		return functional.None[Opened](), err
	}
	s.relations.Store(opened.Store.ModelID(), opened.Relations)
	return functional.Some(opened), nil
}

func (s *stores) findStoreAndModel(ctx context.Context, productID string) (openedRow, bool, error) {
	stmt := table.AuthzStores.SELECT(
		table.AuthzStores.StoreID.AS("opened_row.store_id"),
		table.AuthzStores.ReconciledAt.AS("opened_row.reconciled_at"),
		table.AuthzStores.ModelID.AS("opened_row.legacy_model_id"),
		table.AuthzModels.ModelID.AS("opened_row.model_id"),
	).FROM(
		table.AuthzStores.LEFT_JOIN(
			table.AuthzModels,
			table.AuthzModels.ProductID.EQ(table.AuthzStores.ProductID).
				AND(table.AuthzModels.ModelVersion.EQ(postgres.String(s.version))),
		),
	).WHERE(table.AuthzStores.ProductID.EQ(postgres.String(productID)))

	found, err := transactor.QueryOptional[openedRow](ctx, s.db, stmt)
	if err != nil {
		return openedRow{}, false, err
	}
	row, ok := found.Get()
	return row, ok, nil
}

// resolveModelID returns this release's model ID when recorded. For a store
// already rebuilt on another release (or before #401), it falls back to the
// newest authz_models row or legacy authz_stores.model_id with current=false so
// the checker runs EnsureModel inline instead of triggering a full Reconcile.
func (s *stores) resolveModelID(ctx context.Context, productID string, row openedRow) (string, bool, error) {
	if row.ModelID != nil && *row.ModelID != "" {
		return *row.ModelID, true, nil
	}
	if row.ReconciledAt == nil {
		return "", false, nil
	}
	newest, err := transactor.QueryOptional[model.AuthzModels](ctx, s.db,
		table.AuthzModels.SELECT(table.AuthzModels.ModelID).
			WHERE(table.AuthzModels.ProductID.EQ(postgres.String(productID))).
			ORDER_BY(table.AuthzModels.WrittenAt.DESC()).
			LIMIT(1),
	)
	if err != nil {
		return "", false, err
	}
	if m, ok := newest.Get(); ok && m.ModelID != "" {
		return m.ModelID, false, nil
	}
	if row.LegacyModelID != nil && *row.LegacyModelID != "" {
		return *row.LegacyModelID, false, nil
	}
	return "", false, nil
}

func (s *stores) find(ctx context.Context, productID string) (string, error) {
	found, err := transactor.QueryOptional[model.AuthzStores](ctx, s.db,
		table.AuthzStores.SELECT(table.AuthzStores.StoreID).
			WHERE(table.AuthzStores.ProductID.EQ(postgres.String(productID))),
	)
	if err != nil {
		return "", err
	}
	return found.Map(func(row model.AuthzStores) string { return row.StoreID }).OrElse(""), nil
}

func (s *stores) discard(ctx context.Context, openfga engine.Engine, storeID string) {
	if _, err := openfga.DeleteStore(
		context.WithoutCancel(ctx),
		&openfgav1.DeleteStoreRequest{StoreId: storeID},
	); err != nil {
		s.logger.WarnContext(ctx, "failed to delete an unused openfga store",
			log.Err(err),
			slog.String("store_id", storeID),
		)
	}
}
