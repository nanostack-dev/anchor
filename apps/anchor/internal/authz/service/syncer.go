// Package service keeps each product's OpenFGA store in step with Postgres and
// answers access questions from it.
//
// Postgres stays the source of truth. A write never calls OpenFGA: in its own
// transaction it leaves a mark naming the organization it changed, or the whole
// product for a role or permission change. The syncer replays marks, either
// right before a decision that depends on them or from the background sweeper,
// so a decision always sees every committed write.
package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/pglock"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/rs/zerolog"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/model"
	"anchor/internal/authz/repository"
)

// view is what a decision reads: a product's store, the model its tuples were
// written under, and the catalog that model was built from.
type view struct {
	store   store
	modelID string
	catalog model.Catalog
}

type Syncer struct {
	engine engine.Engine
	repo   repository.Repository
	lock   productLock
	logger zerolog.Logger

	mu     sync.Mutex
	stores map[string]string // product ID → store ID; a store ID never changes
	views  map[string]view   // product ID → last view, valid while its model is the latest
}

func NewSyncer(
	openfga engine.Engine, repo repository.Repository, tx transactor.Transactor, logger zerolog.Logger,
) *Syncer {
	return newSyncer(openfga, repo, advisoryLock{tx: tx}, logger)
}

func newSyncer(openfga engine.Engine, repo repository.Repository, lock productLock, logger zerolog.Logger) *Syncer {
	return &Syncer{
		engine: openfga,
		repo:   repo,
		lock:   lock,
		logger: logger.With().Str("component", "authz_syncer").Logger(),
		stores: map[string]string{},
		views:  map[string]view{},
	}
}

// productLock runs fn while no other sync of the product runs, on any replica.
type productLock interface {
	Do(ctx context.Context, productID string, fn func(context.Context) error) error
}

// advisoryLock holds a Postgres transaction-scoped advisory lock for the
// duration of fn. fn's repository calls join the transaction, so the marks it
// clears are cleared only if the whole sync commits.
type advisoryLock struct {
	tx transactor.Transactor
}

func (l advisoryLock) Do(ctx context.Context, productID string, fn func(context.Context) error) error {
	return l.tx.InTx(ctx, func(txCtx context.Context) error {
		if _, err := transactor.CurrentTx(txCtx).ExecContext(
			txCtx, "SELECT pg_advisory_xact_lock($1)", pglock.KeyHash("authz-sync:"+productID),
		); err != nil {
			return fmt.Errorf("lock authz sync: %w", err)
		}
		return fn(txCtx)
	})
}

// fresh returns a view of the product in which every committed write to the
// organization, and to the product's catalog, is already applied.
func (s *Syncer) fresh(ctx context.Context, productID, organizationID string) (view, error) {
	marks, err := s.repo.PendingFor(ctx, productID, organizationID)
	if err != nil {
		return view{}, err
	}
	storeID, known, err := s.knownStore(ctx, productID)
	if err != nil {
		return view{}, err
	}
	if len(marks) > 0 || !known {
		return s.syncLocked(ctx, productID, func(txCtx context.Context) ([]repository.Mark, error) {
			return s.repo.PendingFor(txCtx, productID, organizationID)
		})
	}
	return s.currentView(ctx, productID, storeID)
}

// SyncProduct replays every pending mark of a product, and creates and fills
// its store if it has none yet. The sweeper calls it.
func (s *Syncer) SyncProduct(ctx context.Context, productID string) error {
	_, err := s.syncLocked(ctx, productID, func(txCtx context.Context) ([]repository.Mark, error) {
		return s.repo.PendingForProduct(txCtx, productID)
	})
	return err
}

// syncLocked replays the marks pending reads under the product's lock, and
// clears them with the lock's transaction. Two syncs of one product could
// otherwise interleave, and the one that read Postgres first could overwrite
// the newer one's tuples.
func (s *Syncer) syncLocked(
	ctx context.Context, productID string, pending func(context.Context) ([]repository.Mark, error),
) (view, error) {
	var result view
	err := s.lock.Do(ctx, productID, func(txCtx context.Context) error {
		storeID, created, err := s.ensureStore(txCtx, productID)
		if err != nil {
			return err
		}
		marks, err := pending(txCtx)
		if err != nil {
			return err
		}
		if result, err = s.replay(txCtx, productID, storeID, created, marks); err != nil {
			return err
		}
		for _, mark := range marks {
			if clearErr := s.repo.Clear(txCtx, mark); clearErr != nil {
				return clearErr
			}
		}
		return nil
	})
	if err != nil {
		return view{}, err
	}
	s.remember(productID, result)
	return result, nil
}

// replay brings the model up to date, then every organization a mark names, or
// all of them when the store is new or a mark covers the whole product.
func (s *Syncer) replay(
	ctx context.Context, productID, storeID string, newStore bool, marks []repository.Mark,
) (view, error) {
	current, err := s.syncModel(ctx, productID, storeID)
	if err != nil {
		return view{}, err
	}
	organizationIDs := make([]string, 0, len(marks))
	wholeProduct := newStore
	for _, mark := range marks {
		wholeProduct = wholeProduct || mark.IsProductWide()
		organizationIDs = append(organizationIDs, mark.OrganizationID)
	}
	if wholeProduct {
		if organizationIDs, err = s.allOrganizations(ctx, current, productID); err != nil {
			return view{}, err
		}
	}
	for _, organizationID := range organizationIDs {
		if syncErr := s.syncOrganization(ctx, current, productID, organizationID); syncErr != nil {
			return view{}, syncErr
		}
	}
	return current, nil
}

// syncModel builds the product's model from its catalog and makes it the
// store's latest.
func (s *Syncer) syncModel(ctx context.Context, productID, storeID string) (view, error) {
	catalog, err := s.repo.Catalog(ctx, productID)
	if err != nil {
		return view{}, err
	}
	built, err := model.Build(catalog)
	if err != nil {
		return view{}, err
	}
	target := store{engine: s.engine, id: storeID}
	modelID, err := target.ensureModel(ctx, built)
	if err != nil {
		return view{}, err
	}
	return view{store: target, modelID: modelID, catalog: catalog}, nil
}

// syncOrganization makes the store hold exactly the tuples the organization
// implies: those of the organization itself and of every workspace linked to it
// in Postgres or, for a deleted workspace, still linked in the store.
func (s *Syncer) syncOrganization(ctx context.Context, current view, productID, organizationID string) error {
	state, err := s.repo.OrganizationState(ctx, productID, organizationID)
	if err != nil {
		return err
	}
	stored, err := s.storedTuples(ctx, current.store, state)
	if err != nil {
		return err
	}
	changes := model.Diff(stored, state.Tuples(current.catalog))
	if changes.IsEmpty() {
		return nil
	}
	s.logger.Debug().Str("product_id", productID).Str("organization_id", organizationID).
		Int("writes", len(changes.Writes)).Int("deletes", len(changes.Deletes)).
		Msg("syncing organization tuples")
	return current.store.apply(ctx, current.modelID, changes)
}

func (s *Syncer) storedTuples(ctx context.Context, target store, state model.OrganizationState) ([]model.Tuple, error) {
	organization := model.Organization(state.OrganizationID)
	tuples, err := target.read(ctx, &openfgav1.ReadRequestTupleKey{Object: organization})
	if err != nil {
		return nil, err
	}
	linked, err := target.read(ctx, &openfgav1.ReadRequestTupleKey{
		Object: model.TypeWorkspace + ":", Relation: model.RelationOrganization, User: organization,
	})
	if err != nil {
		return nil, err
	}
	workspaces := map[string]bool{}
	for _, link := range linked {
		workspaces[link.Object] = true
	}
	for _, workspace := range state.Workspaces {
		workspaces[model.Workspace(workspace.ID)] = true
	}
	for workspace := range workspaces {
		held, readErr := target.read(ctx, &openfgav1.ReadRequestTupleKey{Object: workspace})
		if readErr != nil {
			return nil, readErr
		}
		tuples = append(tuples, held...)
	}
	return tuples, nil
}

// allOrganizations lists the organizations in Postgres and those the store
// still holds, so a product-wide sync also clears deleted organizations.
func (s *Syncer) allOrganizations(ctx context.Context, current view, productID string) ([]string, error) {
	ids, err := s.repo.OrganizationIDs(ctx, productID)
	if err != nil {
		return nil, err
	}
	stored, err := current.store.read(ctx, &openfgav1.ReadRequestTupleKey{
		Object: model.TypeOrganization + ":", Relation: model.RelationProduct, User: model.Product(productID),
	})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	for _, tuple := range stored {
		if id := model.ObjectID(tuple.Object); !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// currentView returns the cached view while its model is still the store's
// latest. Another replica may have synced a catalog change meanwhile; a new
// latest model is how this replica learns it.
func (s *Syncer) currentView(ctx context.Context, productID, storeID string) (view, error) {
	target := store{engine: s.engine, id: storeID}
	latest, err := target.latestModel(ctx)
	if err != nil {
		return view{}, err
	}
	s.mu.Lock()
	cached, ok := s.views[productID]
	s.mu.Unlock()
	if ok && latest != nil && cached.modelID == latest.GetId() {
		return cached, nil
	}
	current, err := s.syncModel(ctx, productID, storeID)
	if err != nil {
		return view{}, err
	}
	s.remember(productID, current)
	return current, nil
}

func (s *Syncer) remember(productID string, current view) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.views[productID] = current
	s.stores[productID] = current.store.id
}

func (s *Syncer) knownStore(ctx context.Context, productID string) (string, bool, error) {
	s.mu.Lock()
	storeID, ok := s.stores[productID]
	s.mu.Unlock()
	if ok {
		return storeID, true, nil
	}
	found, err := s.repo.FindStoreID(ctx, productID)
	if err != nil {
		return "", false, err
	}
	return found.OrElse(""), found.IsPresent(), nil
}

// ensureStore returns the product's store, creating it on first use. It runs
// under the product's sync lock, so two replicas never create two stores.
func (s *Syncer) ensureStore(ctx context.Context, productID string) (string, bool, error) {
	found, err := s.repo.FindStoreID(ctx, productID)
	if err != nil {
		return "", false, err
	}
	if found.IsPresent() {
		return found.Value(), false, nil
	}
	created, err := s.engine.CreateStore(ctx, &openfgav1.CreateStoreRequest{Name: productID})
	if err != nil {
		return "", false, fmt.Errorf("create openfga store: %w", err)
	}
	storeID, err := s.repo.SaveStoreID(ctx, productID, created.GetId())
	if err != nil {
		return "", false, err
	}
	s.logger.Info().Str("product_id", productID).Str("store_id", storeID).Msg("created openfga store")
	return storeID, true, nil
}
