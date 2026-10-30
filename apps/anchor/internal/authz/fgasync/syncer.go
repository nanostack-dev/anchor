package fgasync

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/rs/zerolog"

	"anchor/internal/authz/fga"
)

const (
	// A sync waits this long, at most, for another sync of its Product.
	lockWait     = 3 * time.Second
	lockRetryMin = 10 * time.Millisecond
	lockRetryMax = 200 * time.Millisecond
	// lockRetryGrowth multiplies the wait after each busy attempt.
	lockRetryGrowth = 2
	maxNameLength   = 64
)

var (
	// ErrLockBusy reports that another sync of the Product held the lock for the
	// whole wait. The marks stay, and the sweeper replays them.
	ErrLockBusy = errors.New("authz sync lock busy")
	// ErrInvalidScope is returned for a stale key that is empty.
	ErrInvalidScope = errors.New("invalid authz sync scope")
	// errModelStale reports a catalog that changed since the store's model; a
	// key sync then writes the model under the exclusive lock first.
	errModelStale = errors.New("authz model stale")
)

// Deps is what a Syncer needs. Every field is required except Now.
type Deps struct {
	Engines    Engines
	Stores     Stores
	Repository Repository
	Locker     Locker
	Models     ModelSource
	Sources    []Source
	Logger     zerolog.Logger
	// Now defaults to time.Now.
	Now func() time.Time
}

// Syncer keeps every Product's store in step with its sources.
type Syncer struct {
	engines  Engines
	stores   Stores
	repo     Repository
	locker   Locker
	models   ModelSource
	sources  []Source
	byName   map[string]Source
	compiled *modelCache
	logger   zerolog.Logger
	now      func() time.Time
	timing   timing
	// storesToken continues the walk over OpenFGA's stores between sweeps.
	storesToken string
}

// New checks the sources: each needs a name of at most 64 characters,
// distinct from every other.
func New(deps Deps) (*Syncer, error) {
	byName, err := sourcesByName(deps.Sources)
	if err != nil {
		return nil, err
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &Syncer{
		engines: deps.Engines, stores: deps.Stores, repo: deps.Repository, locker: deps.Locker,
		models: deps.Models, sources: deps.Sources, byName: byName,
		compiled: newModelCache(),
		logger:   deps.Logger.With().Str("component", "authz_syncer").Logger(),
		now:      now,
		timing:   defaultTiming(),
	}, nil
}

func sourcesByName(sources []Source) (map[string]Source, error) {
	byName := map[string]Source{}
	for _, p := range sources {
		name := p.Name()
		if name == "" || len(name) > maxNameLength {
			return nil, fmt.Errorf("authz source name %q must hold 1 to %d characters", name, maxNameLength)
		}
		if _, taken := byName[name]; taken {
			return nil, fmt.Errorf("two authz sources are named %q", name)
		}
		byName[name] = p
	}
	return byName, nil
}

// Sync brings the scope's tuples, and the Product's model, in step with the
// tables, and clears the marks it covered. A scope naming a source the
// syncer does not know, such as one renamed since its mark was left, syncs
// the whole Product.
func (s *Syncer) Sync(ctx context.Context, scope Scope) error {
	if scope.IsWholeProduct() {
		return s.syncAll(ctx, scope.ProductID, s.sources, scope)
	}
	p, known := s.byName[scope.Source]
	switch {
	case !known:
		return s.syncAll(ctx, scope.ProductID, s.sources, WholeProduct(scope.ProductID))
	case scope.IsEveryKey():
		return s.syncAll(ctx, scope.ProductID, []Source{p}, scope)
	default:
		return s.syncKey(ctx, p, scope)
	}
}

// EnsureModel writes the Product's model when it changed since the model its
// store follows, such as after a release that compiles the same catalog into
// a different model. The tuples stay: a new model version only adds relations
// computed from them.
func (s *Syncer) EnsureModel(ctx context.Context, productID string) error {
	source, _, recorded, err := s.modelState(ctx, productID)
	if err != nil {
		return err
	}
	if recorded.Hash == source.Hash && recorded.ID != "" {
		return nil
	}
	engine, storeID, err := s.open(ctx, productID)
	if err != nil {
		return err
	}
	return s.exclusive(ctx, productID, func(txCtx context.Context) error {
		_, lockedErr := s.ensureModel(txCtx, engine, storeID, productID)
		return lockedErr
	})
}

// Reconcile syncs the whole Product, then deletes every tuple no source
// claims, such as the grants of a role deleted without an event.
func (s *Syncer) Reconcile(ctx context.Context, productID string) error {
	exists, err := s.repo.ProductExists(ctx, productID)
	if err != nil || !exists {
		return err
	}
	if err = s.Sync(ctx, WholeProduct(productID)); err != nil {
		return err
	}
	engine, storeID, err := s.open(ctx, productID)
	if err != nil {
		return err
	}
	return s.exclusive(ctx, productID, func(txCtx context.Context) error {
		current, lockedErr := s.ensureModel(txCtx, engine, storeID, productID)
		if lockedErr != nil {
			return lockedErr
		}
		var desired []fga.Tuple
		for _, p := range s.sources {
			keys, keysErr := p.Keys(txCtx, productID)
			if keysErr != nil {
				return keysErr
			}
			for _, key := range keys {
				implied, desiredErr := s.desired(txCtx, current, p, productID, key)
				if desiredErr != nil {
					return desiredErr
				}
				desired = append(desired, implied...)
			}
		}
		stored, lockedErr := current.store.Read(txCtx, fga.Filter{})
		if lockedErr != nil {
			return lockedErr
		}
		changes := fga.Diff(stored, desired)
		if lockedErr = current.store.Apply(txCtx, changes); lockedErr != nil {
			return lockedErr
		}
		if !changes.IsEmpty() {
			s.logger.Info().Str("product_id", productID).Int("writes", len(changes.Writes)).
				Int("deletes", len(changes.Deletes)).Msg("reconcile repaired the relationship store")
		}
		return s.repo.SetReconciled(txCtx, productID, s.now())
	})
}

// target is a Product's store at its current model, and what that model defines.
type target struct {
	store     fga.Store
	relations fga.Relations
}

// syncKey syncs one key beside the syncs of the Product's other keys. It reads
// the marks before it reads the key, so a write landing meanwhile bumps the
// revision and ClearIfUnchanged keeps its mark. A model to write first gets
// its own exclusive step.
func (s *Syncer) syncKey(ctx context.Context, p Source, scope Scope) error {
	engine, storeID, err := s.open(ctx, scope.ProductID)
	if err != nil {
		return err
	}
	sync := func(txCtx context.Context) error {
		marks, lockedErr := s.repo.Pending(txCtx, scope.ProductID)
		if lockedErr != nil {
			return lockedErr
		}
		current, lockedErr := s.currentModel(txCtx, engine, storeID, scope.ProductID)
		if lockedErr != nil {
			return lockedErr
		}
		if lockedErr = s.syncOne(txCtx, current, p, scope.ProductID, scope.Key); lockedErr != nil {
			return lockedErr
		}
		return s.clearCovered(txCtx, marks, scope)
	}
	err = s.keyLocked(ctx, scope, sync)
	if !errors.Is(err, errModelStale) {
		return err
	}
	if err = s.exclusive(ctx, scope.ProductID, func(txCtx context.Context) error {
		_, modelErr := s.ensureModel(txCtx, engine, storeID, scope.ProductID)
		return modelErr
	}); err != nil {
		return err
	}
	return s.keyLocked(ctx, scope, sync)
}

// syncAll syncs every key of the sources, each under its own key lock so no
// write waits behind a whole Product, then clears the marks the covering scope
// covers among those it read first. Keys come from the tables
// and from the marks, since a deleted key is gone from the tables.
func (s *Syncer) syncAll(ctx context.Context, productID string, sources []Source, covering Scope) error {
	engine, storeID, err := s.open(ctx, productID)
	if err != nil {
		return err
	}
	var marks []Mark
	var current target
	if err = s.exclusive(ctx, productID, func(txCtx context.Context) error {
		var lockedErr error
		if marks, lockedErr = s.repo.Pending(txCtx, productID); lockedErr != nil {
			return lockedErr
		}
		current, lockedErr = s.ensureModel(txCtx, engine, storeID, productID)
		return lockedErr
	}); err != nil {
		return err
	}
	for _, p := range sources {
		keys, keysErr := s.keysToSync(ctx, p, productID, marks)
		if keysErr != nil {
			return keysErr
		}
		for _, key := range keys {
			keyScope := Scope{ProductID: productID, Source: p.Name(), Key: key}
			if err = s.keyLocked(ctx, keyScope, func(txCtx context.Context) error {
				return s.syncOne(txCtx, current, p, productID, key)
			}); err != nil {
				return err
			}
		}
	}
	return s.exclusive(ctx, productID, func(txCtx context.Context) error {
		return s.clearCovered(txCtx, marks, covering)
	})
}

func (s *Syncer) keysToSync(ctx context.Context, p Source, productID string, marks []Mark) ([]string, error) {
	keys, err := p.Keys(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("authz source %s: keys: %w", p.Name(), err)
	}
	keys = slices.Clone(keys)
	for _, mark := range marks {
		if mark.Source == p.Name() && !mark.IsEveryKey() && !slices.Contains(keys, mark.Key) {
			keys = append(keys, mark.Key)
		}
	}
	return keys, nil
}

// syncOne makes the store hold exactly what the key implies, among the tuples
// the key owns.
func (s *Syncer) syncOne(ctx context.Context, current target, p Source, productID, key string) error {
	desired, err := s.desired(ctx, current, p, productID, key)
	if err != nil {
		return err
	}
	stored, err := p.Stored(ctx, current.store, key)
	if err != nil {
		return fmt.Errorf("authz source %s: stored tuples of %q: %w", p.Name(), key, err)
	}
	return current.store.Apply(ctx, fga.Diff(stored, desired))
}

// desired is what the key implies, less what the model does not accept: a
// permission deleted from the catalog leaves no relation to hold it, and a
// parent type removed from a resource type no longer takes its parent tuple.
func (s *Syncer) desired(
	ctx context.Context,
	current target,
	p Source,
	productID, key string,
) ([]fga.Tuple, error) {
	desired, err := p.Desired(ctx, productID, key)
	if err != nil {
		return nil, fmt.Errorf("authz source %s: desired tuples of %q: %w", p.Name(), key, err)
	}
	return functional.Slice(desired).Filter(current.relations.Accepts), nil
}

func (s *Syncer) clearCovered(ctx context.Context, marks []Mark, covering Scope) error {
	for _, mark := range marks {
		if !covering.covers(mark.Scope) {
			continue
		}
		if err := s.repo.ClearIfUnchanged(ctx, mark); err != nil {
			return err
		}
	}
	return nil
}

// exclusive runs fn holding the Product's sync lock exclusively.
func (s *Syncer) exclusive(ctx context.Context, productID string, fn func(context.Context) error) error {
	return s.retryLock(ctx, func() (bool, error) { return s.locker.TryLocked(ctx, productID, fn) })
}

// keyLocked runs fn holding the key's lock, beside the Product's other keys.
func (s *Syncer) keyLocked(ctx context.Context, scope Scope, fn func(context.Context) error) error {
	return s.retryLock(ctx, func() (bool, error) {
		return s.locker.TryKeyLocked(ctx, scope.ProductID, scope.Source+":"+scope.Key, fn)
	})
}

// retryLock retries a busy lock with a growing backoff, outside any
// transaction, until the wait runs out.
func (s *Syncer) retryLock(ctx context.Context, try func() (bool, error)) error {
	deadline := time.Now().Add(s.timing.lockWait)
	backoff := s.timing.lockRetryMin
	for {
		acquired, err := try()
		if err != nil || acquired {
			return err
		}
		if time.Now().After(deadline) {
			return ErrLockBusy
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(lockRetryGrowth*backoff, s.timing.lockRetryMax)
	}
}

// open returns the engine and the Product's store, outside any transaction: a
// store created here commits on its own.
func (s *Syncer) open(ctx context.Context, productID string) (fga.Engine, string, error) {
	engine, err := s.engines.Get()
	if err != nil {
		return nil, "", err
	}
	storeID, err := s.stores.StoreID(ctx, productID)
	return engine, storeID, err
}

// currentModel returns the target a key sync writes under, or errModelStale
// when the model changed since the one the store follows.
func (s *Syncer) currentModel(ctx context.Context, engine fga.Engine, storeID, productID string) (target, error) {
	source, compiled, recorded, err := s.modelState(ctx, productID)
	if err != nil {
		return target{}, err
	}
	if recorded.Hash != source.Hash || recorded.ID == "" {
		return target{}, errModelStale
	}
	return target{store: fga.NewStore(engine, storeID, recorded.ID), relations: compiled.relations}, nil
}

func (s *Syncer) modelState(ctx context.Context, productID string) (Model, compiledModel, RecordedModel, error) {
	source, err := s.models.Model(ctx, productID)
	if err != nil {
		return Model{}, compiledModel{}, RecordedModel{}, fmt.Errorf("authz model of %s: %w", productID, err)
	}
	compiled, err := s.compiled.get(source)
	if err != nil {
		return Model{}, compiledModel{}, RecordedModel{}, err
	}
	recorded, err := s.repo.RecordedModel(ctx, productID)
	return source, compiled, recorded, err
}

// ensureModel writes the Product's model when it changed since the model the
// store follows. It runs under the exclusive lock.
func (s *Syncer) ensureModel(ctx context.Context, engine fga.Engine, storeID, productID string) (target, error) {
	source, compiled, recorded, err := s.modelState(ctx, productID)
	if err != nil {
		return target{}, err
	}
	if recorded.Hash != source.Hash || recorded.ID == "" {
		modelID, writeErr := fga.WriteModel(ctx, engine, storeID, compiled.proto)
		if writeErr != nil {
			return target{}, writeErr
		}
		recorded = RecordedModel{ID: modelID, Hash: source.Hash}
		if err = s.repo.SetModel(ctx, productID, recorded); err != nil {
			return target{}, err
		}
	}
	return target{store: fga.NewStore(engine, storeID, recorded.ID), relations: compiled.relations}, nil
}
