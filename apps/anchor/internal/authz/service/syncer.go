// Package service keeps each product's OpenFGA store in step with Postgres and
// answers access questions from it.
//
// Postgres stays the source of truth. A write never calls OpenFGA: in its own
// transaction it leaves a mark naming the organization it changed, or a
// product scope (see repository.ModelScope and repository.ProductScope). The
// syncer replays marks, right before a decision that depends on them or from
// the background sweeper, so a decision always sees every committed write.
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/pgkit/pglock"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/rs/zerolog"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/model"
	"anchor/internal/authz/repository"
)

const (
	// syncTimeout bounds one sync. It runs detached from the request, so a
	// client that hangs up does not roll back work the next caller would redo.
	syncTimeout = 30 * time.Second
	// A decision that finds its product's sync busy waits outside any
	// transaction, so waiters hold no pooled connection.
	lockRetryMin = 10 * time.Millisecond
	lockRetryMax = 200 * time.Millisecond
	// lockRetryGrowth multiplies the wait after each busy attempt.
	lockRetryGrowth = 2
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
	// TryDo reports false, without running fn, when another sync holds the lock.
	TryDo(ctx context.Context, productID string, fn func(context.Context) error) (bool, error)
}

// advisoryLock holds a Postgres transaction-scoped advisory lock for the
// duration of fn. fn's repository calls join the transaction, so the marks it
// clears are cleared only if the whole sync commits.
type advisoryLock struct {
	tx transactor.Transactor
}

var errLockBusy = errors.New("authz sync lock busy")

func (l advisoryLock) TryDo(ctx context.Context, productID string, fn func(context.Context) error) (bool, error) {
	err := l.tx.InTx(ctx, func(txCtx context.Context) error {
		acquired, err := pglock.TryAdvisoryXactLock(txCtx, transactor.CurrentTx(txCtx), "authz-sync:"+productID)
		if err != nil {
			return fmt.Errorf("lock authz sync: %w", err)
		}
		if !acquired {
			return errLockBusy
		}
		return fn(txCtx)
	})
	if errors.Is(err, errLockBusy) {
		return false, nil
	}
	return err == nil, err
}

// fresh returns a view of the product in which every committed write to the
// organization, and to the product's catalog, is already applied.
func (s *Syncer) fresh(ctx context.Context, productID, organizationID string) (view, error) {
	pending := func(txCtx context.Context) ([]repository.Mark, error) {
		return s.repo.PendingFor(txCtx, productID, organizationID)
	}
	backoff := lockRetryMin
	for {
		current, ready, err := s.readyView(ctx, productID, organizationID)
		if err != nil || ready {
			return current, err
		}
		current, synced, err := s.trySync(ctx, productID, pending, false)
		if err != nil || synced {
			return current, err
		}
		select {
		case <-ctx.Done():
			return view{}, ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(lockRetryGrowth*backoff, lockRetryMax)
	}
}

// readyView returns the view without syncing when nothing the decision depends
// on is pending and the store's latest model matches the catalog.
func (s *Syncer) readyView(ctx context.Context, productID, organizationID string) (view, bool, error) {
	marks, err := s.repo.PendingFor(ctx, productID, organizationID)
	if err != nil || len(marks) > 0 {
		return view{}, false, err
	}
	storeID, err := s.knownStore(ctx, productID)
	if err != nil || storeID.IsAbsent() {
		return view{}, false, err
	}
	target := store{engine: s.engine, id: storeID.Value()}
	latest, err := target.latestModel(ctx)
	if err != nil || latest == nil {
		return view{}, false, err
	}
	if cached, ok := s.cachedView(productID); ok && cached.modelID == latest.GetId() {
		return cached, true, nil
	}
	// Another replica wrote the latest model. Adopt it only if it is the model
	// of the catalog as it is now; otherwise sync under the lock, which is the
	// only place models are written, so an older catalog can never win.
	catalog, err := s.repo.Catalog(ctx, productID)
	if err != nil {
		return view{}, false, err
	}
	built, err := model.Build(catalog)
	if err != nil || !sameModel(latest, built.Proto) {
		return view{}, false, err
	}
	current := view{store: target, modelID: latest.GetId(), catalog: catalog}
	s.remember(productID, current)
	return current, true, nil
}

// SyncProduct replays every pending mark of a product, product scope included,
// and creates and fills the store of a product that has none. It skips a
// product another sync holds. The sweeper calls it.
func (s *Syncer) SyncProduct(ctx context.Context, productID string) error {
	_, _, err := s.trySync(ctx, productID, func(txCtx context.Context) ([]repository.Mark, error) {
		return s.repo.PendingForProduct(txCtx, productID)
	}, false)
	return err
}

// Reconcile rewrites every organization of a product from Postgres, whatever
// the marks say, and records when. It repairs drift from writes that bypassed
// the services, such as a data fix in SQL or an API key that expired.
func (s *Syncer) Reconcile(ctx context.Context, productID string) error {
	_, synced, err := s.trySync(ctx, productID, func(txCtx context.Context) ([]repository.Mark, error) {
		return s.repo.PendingForProduct(txCtx, productID)
	}, true)
	if err != nil || !synced {
		return err
	}
	return s.repo.Reconciled(ctx, productID)
}

// trySync replays the marks pending reads under the product's lock and clears
// them with the lock's transaction. Two syncs of one product could otherwise
// interleave, and the one that read Postgres first could overwrite the newer
// one's tuples or model.
func (s *Syncer) trySync(
	ctx context.Context, productID string,
	pending func(context.Context) ([]repository.Mark, error), wholeProduct bool,
) (view, bool, error) {
	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), syncTimeout)
	defer cancel()

	var result view
	acquired, err := s.lock.TryDo(work, productID, func(txCtx context.Context) error {
		// The store is recorded outside the lock's transaction, so a sync that
		// fails afterwards still leaves it known instead of orphaned.
		storeID, created, err := s.ensureStore(work, productID)
		if err != nil {
			return err
		}
		marks, err := pending(txCtx)
		if err != nil {
			return err
		}
		if result, err = s.replay(txCtx, productID, storeID, wholeProduct || created, marks); err != nil {
			return err
		}
		for _, mark := range marks {
			if clearErr := s.repo.Clear(txCtx, mark); clearErr != nil {
				return clearErr
			}
		}
		return nil
	})
	if err != nil || !acquired {
		return view{}, false, err
	}
	s.remember(productID, result)
	return result, true, nil
}

// replay rebuilds the model, then every organization a mark names, or all of
// them when asked or when a mark covers the whole product.
func (s *Syncer) replay(
	ctx context.Context, productID, storeID string, wholeProduct bool, marks []repository.Mark,
) (view, error) {
	current, err := s.syncModel(ctx, productID, storeID)
	if err != nil {
		return view{}, err
	}
	var organizationIDs []string
	for _, mark := range marks {
		switch {
		case mark.IsProduct():
			wholeProduct = true
		case mark.IsModel():
		default:
			organizationIDs = append(organizationIDs, mark.OrganizationID)
		}
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
// store's latest. Only a sync holding the product's lock calls it.
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

func (s *Syncer) cachedView(productID string) (view, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cached, ok := s.views[productID]
	return cached, ok
}

func (s *Syncer) remember(productID string, current view) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.views[productID] = current
	s.stores[productID] = current.store.id
}

func (s *Syncer) knownStore(ctx context.Context, productID string) (functional.Option[string], error) {
	s.mu.Lock()
	storeID, ok := s.stores[productID]
	s.mu.Unlock()
	if ok {
		return functional.Some(storeID), nil
	}
	return s.repo.FindStoreID(ctx, productID)
}

// ensureStore returns the product's store, creating it on first use. It runs
// under the product's lock, so two replicas never create two stores, and it
// adopts a store left by an earlier attempt that never got recorded.
func (s *Syncer) ensureStore(ctx context.Context, productID string) (string, bool, error) {
	found, err := s.repo.FindStoreID(ctx, productID)
	if err != nil {
		return "", false, err
	}
	if found.IsPresent() {
		return found.Value(), false, nil
	}
	storeID, err := s.storeNamed(ctx, productID)
	if err != nil {
		return "", false, err
	}
	if storeID == "" {
		created, createErr := s.engine.CreateStore(ctx, &openfgav1.CreateStoreRequest{Name: productID})
		if createErr != nil {
			return "", false, fmt.Errorf("create openfga store: %w", createErr)
		}
		storeID = created.GetId()
	}
	if storeID, err = s.repo.SaveStoreID(ctx, productID, storeID); err != nil {
		return "", false, err
	}
	s.logger.Info().Str("product_id", productID).Str("store_id", storeID).Msg("recorded openfga store")
	return storeID, true, nil
}

func (s *Syncer) storeNamed(ctx context.Context, productID string) (string, error) {
	stores, err := s.engine.ListStores(ctx, &openfgav1.ListStoresRequest{Name: productID})
	if err != nil {
		return "", fmt.Errorf("list openfga stores: %w", err)
	}
	for _, candidate := range stores.GetStores() {
		if candidate.GetName() == productID {
			return candidate.GetId(), nil
		}
	}
	return "", nil
}
