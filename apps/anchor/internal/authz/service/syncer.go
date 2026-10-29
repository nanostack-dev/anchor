// Package service keeps each Product's OpenFGA store in step with Anchor's
// tables (ADR-0022 E5). A write leaves a mark in its own transaction; right
// after it commits, the request replays its Organization's marks, so the next
// request already sees the change. The sweeper replays what is left within
// seconds and rebuilds each Product in full every 15 minutes.
package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/pgkit/pglock"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/model"
	"anchor/internal/authz/repository"
	"anchor/internal/authz/store"
	"anchor/internal/domain/resourcetype"
	rtservice "anchor/internal/resourcetype/service"
)

const (
	// OpenFGA accepts at most this many writes and deletes in one request.
	writeBatch = 100
	readPage   = 100
	// A sync waits this long, at most, for another sync of its Product.
	lockWait     = 3 * time.Second
	lockRetryMin = 10 * time.Millisecond
	lockRetryMax = 200 * time.Millisecond
	// lockRetryGrowth multiplies the wait after each busy attempt.
	lockRetryGrowth = 2
)

// errLockBusy reports that another sync of the Product held the lock for the
// whole wait; the mark stays, and the sweeper replays it.
var errLockBusy = errors.New("authz sync lock busy")

type Syncer interface {
	// SyncOrganization brings the Organization's tuples, and the Product's
	// model, in step with the tables, and clears the marks it covered.
	SyncOrganization(ctx context.Context, productID, organizationID string) error
	// SyncProduct does it for the model and every Organization of the Product.
	SyncProduct(ctx context.Context, productID string) error
	// Reconcile is SyncProduct plus the removal of every tuple no
	// Organization implies, such as the grants of a deleted role.
	Reconcile(ctx context.Context, productID string) error
}

type syncer struct {
	engines       engine.Provider
	stores        store.Stores
	repo          repository.Repository
	resourceTypes rtservice.Service
	tx            transactor.Transactor
	logger        zerolog.Logger
	now           func() time.Time
}

func NewSyncer(
	engines engine.Provider,
	stores store.Stores,
	repo repository.Repository,
	resourceTypes rtservice.Service,
	tx transactor.Transactor,
	logger zerolog.Logger,
) Syncer {
	return &syncer{
		engines: engines, stores: stores, repo: repo, resourceTypes: resourceTypes, tx: tx,
		logger: logger.With().Str("component", "authz_syncer").Logger(),
		now:    time.Now,
	}
}

// fgaStore is a Product's store and the model its tuples follow.
type fgaStore struct {
	openfga engine.Engine
	storeID string
	modelID string
	catalog model.Catalog
}

func (s *syncer) SyncOrganization(ctx context.Context, productID, organizationID string) error {
	openfga, storeID, storeErr := s.openStore(ctx, productID)
	if storeErr != nil {
		return storeErr
	}
	return s.locked(ctx, productID, func(txCtx context.Context) error {
		// Read before syncing: a write landing meanwhile bumps the revision,
		// so ClearIfUnchanged keeps its mark.
		marks, err := s.repo.Pending(txCtx, productID)
		if err != nil {
			return err
		}
		fga, err := s.ensureModel(txCtx, openfga, storeID, productID)
		if err != nil {
			return err
		}
		if err = s.syncOrganization(txCtx, fga, productID, organizationID); err != nil {
			return err
		}
		mark, found := functional.Slice(marks).FindFirst(func(m repository.Mark) bool {
			return m.OrganizationID == organizationID
		}).Get()
		if !found {
			return nil
		}
		return s.repo.ClearIfUnchanged(txCtx, mark)
	})
}

func (s *syncer) SyncProduct(ctx context.Context, productID string) error {
	_, err := s.syncProduct(ctx, productID)
	return err
}

// syncProduct syncs every Organization, each under its own hold of the lock so
// no write waits behind a whole Product, then clears the marks it read first.
func (s *syncer) syncProduct(ctx context.Context, productID string) (fgaStore, error) {
	openfga, storeID, err := s.openStore(ctx, productID)
	if err != nil {
		return fgaStore{}, err
	}
	var marks []repository.Mark
	var fga fgaStore
	if err = s.locked(ctx, productID, func(txCtx context.Context) error {
		var lockedErr error
		if marks, lockedErr = s.repo.Pending(txCtx, productID); lockedErr != nil {
			return lockedErr
		}
		fga, lockedErr = s.ensureModel(txCtx, openfga, storeID, productID)
		return lockedErr
	}); err != nil {
		return fgaStore{}, err
	}
	existing, err := s.repo.OrganizationIDs(ctx, productID)
	if err != nil {
		return fgaStore{}, err
	}
	for _, organizationID := range organizationsToSync(existing, marks) {
		if err = s.locked(ctx, productID, func(txCtx context.Context) error {
			return s.syncOrganization(txCtx, fga, productID, organizationID)
		}); err != nil {
			return fgaStore{}, err
		}
	}
	return fga, s.locked(ctx, productID, func(txCtx context.Context) error {
		if heldErr := s.syncManagedRoles(txCtx, fga, productID); heldErr != nil {
			return heldErr
		}
		for _, mark := range marks {
			if clearErr := s.repo.ClearIfUnchanged(txCtx, mark); clearErr != nil {
				return clearErr
			}
		}
		return nil
	})
}

// organizationsToSync adds the Organizations only a mark names: a deleted
// Organization is gone from the table, and its tuples must go too.
func organizationsToSync(existing []string, marks []repository.Mark) []string {
	organizations := slices.Clone(existing)
	for _, mark := range marks {
		if !mark.CoversProduct() && !slices.Contains(organizations, mark.OrganizationID) {
			organizations = append(organizations, mark.OrganizationID)
		}
	}
	return organizations
}

func (s *syncer) Reconcile(ctx context.Context, productID string) error {
	exists, existsErr := s.repo.ProductExists(ctx, productID)
	if existsErr != nil || !exists {
		return existsErr
	}
	fga, syncErr := s.syncProduct(ctx, productID)
	if syncErr != nil {
		return syncErr
	}
	return s.locked(ctx, productID, func(txCtx context.Context) error {
		desired, err := s.productTuples(txCtx, fga, productID)
		if err != nil {
			return err
		}
		stored, err := s.readAll(txCtx, fga, nil)
		if err != nil {
			return err
		}
		changes := model.Diff(stored, desired)
		if err = s.apply(txCtx, fga, changes); err != nil {
			return err
		}
		if !changes.IsEmpty() {
			s.logger.Info().Str("product_id", productID).Int("writes", len(changes.Writes)).
				Int("deletes", len(changes.Deletes)).Msg("reconcile repaired the relationship store")
		}
		return s.repo.SetReconciled(txCtx, productID, s.now())
	})
}

// locked runs fn in a transaction that holds the Product's sync lock, so one
// sync of a Product runs at a time across replicas. A busy lock is retried for
// a short while, outside any transaction.
func (s *syncer) locked(ctx context.Context, productID string, fn func(context.Context) error) error {
	deadline := time.Now().Add(lockWait)
	backoff := lockRetryMin
	for {
		err := s.tx.InTx(ctx, func(txCtx context.Context) error {
			acquired, err := pglock.TryAdvisoryXactLock(txCtx, transactor.CurrentTx(txCtx), "authz-sync:"+productID)
			if err != nil {
				return fmt.Errorf("lock authz sync: %w", err)
			}
			if !acquired {
				return errLockBusy
			}
			return fn(txCtx)
		})
		if !errors.Is(err, errLockBusy) || time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(lockRetryGrowth*backoff, lockRetryMax)
	}
}

// openStore returns the engine and the Product's store, outside any
// transaction: a store created here commits on its own.
func (s *syncer) openStore(ctx context.Context, productID string) (engine.Engine, string, error) {
	openfga, err := s.engines.Get()
	if err != nil {
		return nil, "", err
	}
	storeID, err := s.stores.StoreID(ctx, productID)
	return openfga, storeID, err
}

// ensureModel writes the Product's model when its catalog changed since the
// model the store follows.
func (s *syncer) ensureModel(
	ctx context.Context, openfga engine.Engine, storeID, productID string,
) (fgaStore, error) {
	catalog, err := s.catalog(ctx, productID)
	if err != nil {
		return fgaStore{}, err
	}
	found, err := s.repo.FindStore(ctx, productID)
	if err != nil {
		return fgaStore{}, err
	}
	fga := fgaStore{openfga: openfga, storeID: storeID, catalog: catalog}
	recorded := found.OrElse(repository.Store{})
	if recorded.ModelHash == model.Hash(catalog) && recorded.ModelID != "" {
		fga.modelID = recorded.ModelID
		return fga, nil
	}
	compiled, err := model.Compile(catalog)
	if err != nil {
		return fgaStore{}, err
	}
	written, err := openfga.WriteAuthorizationModel(ctx, &openfgav1.WriteAuthorizationModelRequest{
		StoreId:         storeID,
		SchemaVersion:   compiled.Proto.GetSchemaVersion(),
		TypeDefinitions: compiled.Proto.GetTypeDefinitions(),
		Conditions:      compiled.Proto.GetConditions(),
	})
	if err != nil {
		return fgaStore{}, fmt.Errorf("write authorization model: %w", err)
	}
	fga.modelID = written.GetAuthorizationModelId()
	return fga, s.repo.SetModel(ctx, productID, fga.modelID, compiled.Hash)
}

func (s *syncer) catalog(ctx context.Context, productID string) (model.Catalog, error) {
	tree, err := s.resourceTypes.Tree(ctx, productID)
	if err != nil {
		return model.Catalog{}, err
	}
	permissions, err := s.repo.Permissions(ctx, productID)
	if err != nil {
		return model.Catalog{}, err
	}
	var catalog model.Catalog
	for _, t := range tree.Types() {
		catalog.Types = append(catalog.Types, model.Type{
			Slug: t.Slug, ParentTypes: t.ParentTypes, NestsInItself: t.NestsInItself,
		})
	}
	for _, p := range permissions {
		scope := tree.ByID(p.ResourceTypeID).Map(func(rt resourcetype.ResourceType) string { return rt.Slug }).
			OrElse(resourcetype.OrganizationSlug)
		catalog.Permissions = append(catalog.Permissions, model.Permission{Name: p.Name, Scope: scope})
	}
	return catalog, nil
}

// syncOrganization diffs the tuples an Organization implies with those its
// store holds, and writes the difference.
func (s *syncer) syncOrganization(ctx context.Context, fga fgaStore, productID, organizationID string) error {
	state, err := s.repo.OrganizationState(ctx, productID, organizationID, s.now())
	if err != nil {
		return err
	}
	desired := state.Tuples(fga.catalog)
	stored, err := s.storedOrganizationTuples(ctx, fga, state, desired)
	if err != nil {
		return err
	}
	return s.apply(ctx, fga, model.Diff(stored, desired))
}

// storedOrganizationTuples reads what the store holds for the Organization:
// its object, whose assignment links imply their role tuples, each
// assignment's assignees, and the grants of the roles it owns or its members
// hold. A managed role's grants are kept only where desired, since other
// Organizations may hold that role and an Organization sync must not delete them.
func (s *syncer) storedOrganizationTuples(
	ctx context.Context, fga fgaStore, state model.OrganizationState, desired []model.Tuple,
) ([]model.Tuple, error) {
	links, err := s.readAll(ctx, fga, &openfgav1.ReadRequestTupleKey{
		Object: model.Organization(state.OrganizationID),
	})
	if err != nil {
		return nil, err
	}
	stored := slices.Clone(links)
	roles := map[string]bool{}
	for _, link := range links {
		if link.Relation != model.RelationAssignment {
			continue
		}
		if roleTuple, ok := model.AssignmentRoleTuple(link.User).Get(); ok {
			stored = append(stored, roleTuple)
			roles[model.ObjectID(roleTuple.User)] = true
		}
		assignees, readErr := s.readAll(ctx, fga, &openfgav1.ReadRequestTupleKey{
			Object: link.User, Relation: model.RelationAssignee,
		})
		if readErr != nil {
			return nil, readErr
		}
		stored = append(stored, assignees...)
	}
	owned := map[string]bool{}
	for _, role := range state.Roles {
		roles[role.RowID] = true
		owned[role.RowID] = role.Owned
	}
	for roleRowID := range roles {
		grants, readErr := s.roleGrants(ctx, fga, roleRowID, owned[roleRowID], desired)
		if readErr != nil {
			return nil, readErr
		}
		stored = append(stored, grants...)
	}
	return stored, nil
}

// roleGrants reads a role's stored grants. For a role the Organization does
// not own, it keeps only the desired ones, so the diff never deletes the rest.
func (s *syncer) roleGrants(
	ctx context.Context, fga fgaStore, roleRowID string, owned bool, desired []model.Tuple,
) ([]model.Tuple, error) {
	grants, err := s.readAll(ctx, fga, &openfgav1.ReadRequestTupleKey{Object: model.Role(roleRowID)})
	if err != nil || owned {
		return grants, err
	}
	return functional.Slice(grants).Filter(func(t model.Tuple) bool { return slices.Contains(desired, t) }), nil
}

// syncManagedRoles brings the grants of every managed role a membership holds
// in step: an Organization sync never deletes them, since other Organizations
// may hold the same role.
func (s *syncer) syncManagedRoles(ctx context.Context, fga fgaStore, productID string) error {
	managed, err := s.repo.HeldManagedRoles(ctx, productID)
	if err != nil {
		return err
	}
	for _, held := range managed {
		desired := model.OrganizationState{Exists: true, Roles: []model.RoleGrants{held}}.Tuples(fga.catalog)
		stored, readErr := s.readAll(ctx, fga, &openfgav1.ReadRequestTupleKey{Object: model.Role(held.RowID)})
		if readErr != nil {
			return readErr
		}
		if err = s.apply(ctx, fga, model.Diff(stored, desired)); err != nil {
			return err
		}
	}
	return nil
}

// productTuples is every tuple the Product's Organizations imply.
func (s *syncer) productTuples(ctx context.Context, fga fgaStore, productID string) ([]model.Tuple, error) {
	organizations, err := s.repo.OrganizationIDs(ctx, productID)
	if err != nil {
		return nil, err
	}
	var desired []model.Tuple
	for _, organizationID := range organizations {
		state, stateErr := s.repo.OrganizationState(ctx, productID, organizationID, s.now())
		if stateErr != nil {
			return nil, stateErr
		}
		desired = append(desired, state.Tuples(fga.catalog)...)
	}
	return desired, nil
}

func (s *syncer) readAll(
	ctx context.Context, fga fgaStore, key *openfgav1.ReadRequestTupleKey,
) ([]model.Tuple, error) {
	var tuples []model.Tuple
	token := ""
	for {
		resp, err := fga.openfga.Read(ctx, &openfgav1.ReadRequest{
			StoreId:           fga.storeID,
			TupleKey:          key,
			PageSize:          wrapperspb.Int32(readPage),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("read tuples: %w", err)
		}
		for _, t := range resp.GetTuples() {
			k := t.GetKey()
			tuples = append(tuples, model.Tuple{Object: k.GetObject(), Relation: k.GetRelation(), User: k.GetUser()})
		}
		token = resp.GetContinuationToken()
		if token == "" {
			return tuples, nil
		}
	}
}

// apply writes the changes in batches, deletes first. A tuple already there,
// or already gone, is not an error: a sync that ran halfway before is
// replayed whole.
func (s *syncer) apply(ctx context.Context, fga fgaStore, changes model.Changes) error {
	for !changes.IsEmpty() {
		// Deletes go first, so a sync cut short never leaves a subject with
		// both its old and its new grants.
		deletes := take(&changes.Deletes, writeBatch)
		writes := take(&changes.Writes, writeBatch-len(deletes))
		request := &openfgav1.WriteRequest{StoreId: fga.storeID, AuthorizationModelId: fga.modelID}
		if len(writes) > 0 {
			request.Writes = &openfgav1.WriteRequestWrites{
				TupleKeys:   functional.Slice(writes).Map(toTupleKey),
				OnDuplicate: "ignore",
			}
		}
		if len(deletes) > 0 {
			request.Deletes = &openfgav1.WriteRequestDeletes{
				TupleKeys: functional.Slice(deletes).Map(toTupleKeyWithoutCondition),
				OnMissing: "ignore",
			}
		}
		if _, err := fga.openfga.Write(ctx, request); err != nil {
			return fmt.Errorf("write tuples: %w", err)
		}
	}
	return nil
}

func take(tuples *[]model.Tuple, n int) []model.Tuple {
	n = min(n, len(*tuples))
	batch := (*tuples)[:n]
	*tuples = (*tuples)[n:]
	return batch
}

func toTupleKey(t model.Tuple) *openfgav1.TupleKey {
	return &openfgav1.TupleKey{Object: t.Object, Relation: t.Relation, User: t.User}
}

func toTupleKeyWithoutCondition(t model.Tuple) *openfgav1.TupleKeyWithoutCondition {
	return &openfgav1.TupleKeyWithoutCondition{Object: t.Object, Relation: t.Relation, User: t.User}
}
