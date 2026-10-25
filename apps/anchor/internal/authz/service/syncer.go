// Package service keeps each Product's OpenFGA store in step with Anchor's
// tables (ADR-0022 E5). A write leaves a mark in its own transaction; right
// after it commits, the request replays its Organization's marks, so the next
// request already sees the change. The sweeper replays what is left within
// seconds and rebuilds one Product in full every 15 minutes.
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

// target is a Product's store and the model its tuples follow.
type target struct {
	openfga engine.Engine
	storeID string
	modelID string
	catalog model.Catalog
}

func (s *syncer) SyncOrganization(ctx context.Context, productID, organizationID string) error {
	openfga, storeID, storeErr := s.store(ctx, productID)
	if storeErr != nil {
		return storeErr
	}
	return s.locked(ctx, productID, func(txCtx context.Context) error {
		marks, err := s.repo.Pending(txCtx, productID)
		if err != nil {
			return err
		}
		current, err := s.ensureModel(txCtx, openfga, storeID, productID)
		if err != nil {
			return err
		}
		if err = s.syncOrganization(txCtx, current, productID, organizationID); err != nil {
			return err
		}
		for _, mark := range marks {
			if mark.OrganizationID == organizationID {
				return s.repo.Clear(txCtx, mark)
			}
		}
		return nil
	})
}

func (s *syncer) SyncProduct(ctx context.Context, productID string) error {
	_, err := s.syncProduct(ctx, productID)
	return err
}

// syncProduct syncs every Organization, each under its own hold of the lock so
// no write waits behind a whole Product, then clears the marks it read first.
func (s *syncer) syncProduct(ctx context.Context, productID string) (target, error) {
	openfga, storeID, err := s.store(ctx, productID)
	if err != nil {
		return target{}, err
	}
	var marks []repository.Mark
	var current target
	if err = s.locked(ctx, productID, func(txCtx context.Context) error {
		var lockedErr error
		if marks, lockedErr = s.repo.Pending(txCtx, productID); lockedErr != nil {
			return lockedErr
		}
		current, lockedErr = s.ensureModel(txCtx, openfga, storeID, productID)
		return lockedErr
	}); err != nil {
		return target{}, err
	}
	organizations, err := s.repo.OrganizationIDs(ctx, productID)
	if err != nil {
		return target{}, err
	}
	for _, mark := range marks {
		if mark.OrganizationID != repository.ProductScope && !slices.Contains(organizations, mark.OrganizationID) {
			organizations = append(organizations, mark.OrganizationID)
		}
	}
	for _, organizationID := range organizations {
		if err = s.locked(ctx, productID, func(txCtx context.Context) error {
			return s.syncOrganization(txCtx, current, productID, organizationID)
		}); err != nil {
			return target{}, err
		}
	}
	return current, s.locked(ctx, productID, func(txCtx context.Context) error {
		if heldErr := s.syncHeldTemplates(txCtx, current, productID); heldErr != nil {
			return heldErr
		}
		for _, mark := range marks {
			if clearErr := s.repo.Clear(txCtx, mark); clearErr != nil {
				return clearErr
			}
		}
		return nil
	})
}

func (s *syncer) Reconcile(ctx context.Context, productID string) error {
	exists, existsErr := s.repo.ProductExists(ctx, productID)
	if existsErr != nil || !exists {
		return existsErr
	}
	current, syncErr := s.syncProduct(ctx, productID)
	if syncErr != nil {
		return syncErr
	}
	return s.locked(ctx, productID, func(txCtx context.Context) error {
		desired, err := s.productTuples(txCtx, current, productID)
		if err != nil {
			return err
		}
		stored, err := s.readAll(txCtx, current, nil)
		if err != nil {
			return err
		}
		changes := model.Diff(stored, desired)
		if err = s.apply(txCtx, current, changes); err != nil {
			return err
		}
		if len(changes.Writes)+len(changes.Deletes) > 0 {
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

// store returns the engine and the Product's store, outside any transaction:
// a store created here commits on its own.
func (s *syncer) store(ctx context.Context, productID string) (engine.Engine, string, error) {
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
) (target, error) {
	catalog, err := s.catalog(ctx, productID)
	if err != nil {
		return target{}, err
	}
	found, err := s.repo.FindStore(ctx, productID)
	if err != nil {
		return target{}, err
	}
	current := found.OrElse(repository.Store{})
	if current.ModelHash == model.Hash(catalog) && current.ModelID != "" {
		return target{openfga: openfga, storeID: storeID, modelID: current.ModelID, catalog: catalog}, nil
	}
	compiled, err := model.Compile(catalog)
	if err != nil {
		return target{}, err
	}
	written, err := openfga.WriteAuthorizationModel(ctx, &openfgav1.WriteAuthorizationModelRequest{
		StoreId:         storeID,
		SchemaVersion:   compiled.Proto.GetSchemaVersion(),
		TypeDefinitions: compiled.Proto.GetTypeDefinitions(),
		Conditions:      compiled.Proto.GetConditions(),
	})
	if err != nil {
		return target{}, fmt.Errorf("write authorization model: %w", err)
	}
	if err = s.repo.SetModel(ctx, productID, written.GetAuthorizationModelId(), compiled.Hash); err != nil {
		return target{}, err
	}
	return target{openfga: openfga, storeID: storeID, modelID: written.GetAuthorizationModelId(), catalog: catalog}, nil
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
		scope := resourcetype.OrganizationSlug
		if p.ResourceTypeID != nil {
			scope = tree.ByID(*p.ResourceTypeID).Map(func(rt resourcetype.ResourceType) string { return rt.Slug }).
				OrElse(resourcetype.OrganizationSlug)
		}
		catalog.Permissions = append(catalog.Permissions, model.Permission{Name: p.Name, Scope: scope})
	}
	return catalog, nil
}

// syncOrganization diffs the tuples an Organization implies with those its
// store holds. It reads the Organization's object, whose assignment links
// imply their assignments' tuples, and the roles the Organization owns or its
// members hold. It never deletes the grants of a role another Organization
// may hold: a role template a release before copies assigned.
func (s *syncer) syncOrganization(ctx context.Context, current target, productID, organizationID string) error {
	state, err := s.repo.OrganizationState(ctx, productID, organizationID, s.now())
	if err != nil {
		return err
	}
	desired := state.Tuples(current.catalog)

	stored, err := s.readAll(ctx, current, &openfgav1.ReadRequestTupleKey{
		Object: model.Organization(organizationID),
	})
	if err != nil {
		return err
	}
	roles := map[string]bool{}
	for _, tuple := range stored {
		if tuple.Relation != model.RelationAssignment {
			continue
		}
		implied := model.AssignmentTuples(tuple.User)
		stored = append(stored, implied...)
		for _, t := range implied {
			roles[model.ObjectID(t.User)] = true
		}
		assignees, readErr := s.readAll(ctx, current, &openfgav1.ReadRequestTupleKey{
			Object: tuple.User, Relation: model.RelationAssignee,
		})
		if readErr != nil {
			return readErr
		}
		stored = append(stored, assignees...)
	}
	owned := map[string]bool{}
	for _, role := range state.Roles {
		roles[role.RowID] = true
		if role.Owned {
			owned[role.RowID] = true
		}
	}
	for roleRowID := range roles {
		grants, readErr := s.readAll(ctx, current, &openfgav1.ReadRequestTupleKey{Object: model.Role(roleRowID)})
		if readErr != nil {
			return readErr
		}
		if !owned[roleRowID] {
			grants = functional.Slice(grants).Filter(func(t model.Tuple) bool {
				return slices.Contains(desired, t)
			})
		}
		stored = append(stored, grants...)
	}
	return s.apply(ctx, current, model.Diff(stored, desired))
}

// syncHeldTemplates brings the grants of every role template a membership
// still holds itself in step: an Organization sync never deletes them, since
// other Organizations may hold the same template.
func (s *syncer) syncHeldTemplates(ctx context.Context, current target, productID string) error {
	templates, err := s.repo.HeldTemplates(ctx, productID)
	if err != nil {
		return err
	}
	for _, template := range templates {
		desired := model.OrganizationState{Exists: true, Roles: []model.RoleGrants{template}}.Tuples(current.catalog)
		stored, readErr := s.readAll(ctx, current, &openfgav1.ReadRequestTupleKey{Object: model.Role(template.RowID)})
		if readErr != nil {
			return readErr
		}
		if err = s.apply(ctx, current, model.Diff(stored, desired)); err != nil {
			return err
		}
	}
	return nil
}

// productTuples is every tuple the Product's Organizations imply.
func (s *syncer) productTuples(ctx context.Context, current target, productID string) ([]model.Tuple, error) {
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
		desired = append(desired, state.Tuples(current.catalog)...)
	}
	return desired, nil
}

func (s *syncer) readAll(
	ctx context.Context, current target, key *openfgav1.ReadRequestTupleKey,
) ([]model.Tuple, error) {
	var tuples []model.Tuple
	token := ""
	for {
		resp, err := current.openfga.Read(ctx, &openfgav1.ReadRequest{
			StoreId:           current.storeID,
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
func (s *syncer) apply(ctx context.Context, current target, changes model.Changes) error {
	for len(changes.Writes) > 0 || len(changes.Deletes) > 0 {
		// Deletes go first, so a sync cut short never leaves a subject with
		// both its old and its new grants.
		deletes := take(&changes.Deletes, writeBatch)
		writes := take(&changes.Writes, writeBatch-len(deletes))
		request := &openfgav1.WriteRequest{StoreId: current.storeID, AuthorizationModelId: current.modelID}
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
		if _, err := current.openfga.Write(ctx, request); err != nil {
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
