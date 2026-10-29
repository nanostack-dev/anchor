package service

import (
	"context"
	"time"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/pgkit/pglock"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/repository"
)

const (
	sweepInterval     = 5 * time.Second
	sweepBatchSize    = 100
	reconcileInterval = 15 * time.Minute
	// failureBackoff is how long a Product whose sync failed waits behind the
	// others before the sweeper tries it again.
	failureBackoff = time.Minute
	sweepLockKey   = "authz-sweep"
	// replayTimeout bounds the replay that follows a request's commit; a
	// catalog change replays every Organization of its Product.
	replayTimeout = 30 * time.Second
)

// Sweeper keeps every store converging without a request driving it: it
// replays the marks a request could not, and rebuilds one Product per pass
// once its last rebuild is older than reconcileInterval, repairing drift from
// writes that left no mark. Its state lives in Postgres, so a restart loses
// nothing, and one replica sweeps at a time.
type Sweeper struct {
	syncer  Syncer
	repo    repository.Repository
	engines engine.Provider
	lock    *pglock.Client
	logger  zerolog.Logger
	now     func() time.Time
	// storesToken continues the walk over OpenFGA's stores between sweeps.
	storesToken string
}

func NewSweeper(
	syncer Syncer, repo repository.Repository, engines engine.Provider, lock *pglock.Client, logger zerolog.Logger,
) *Sweeper {
	return &Sweeper{
		syncer: syncer, repo: repo, engines: engines, lock: lock,
		logger: logger.With().Str("component", "authz_sweeper").Logger(),
		now:    time.Now,
	}
}

// Run sweeps until the application stops.
func (s *Sweeper) Run(lc fx.Lifecycle) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(done)
				ticker := time.NewTicker(sweepInterval)
				defer ticker.Stop()
				for {
					s.sweepAlone(ctx)
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
					}
				}
			}()
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			cancel()
			select {
			case <-done:
			case <-stopCtx.Done():
			}
			return nil
		},
	})
}

// sweepAlone runs one pass on one replica at a time: the others skip it.
func (s *Sweeper) sweepAlone(ctx context.Context) {
	if _, err := s.lock.TryWithSessionLock(ctx, sweepLockKey, func(lockCtx context.Context) error {
		s.Sweep(lockCtx)
		return nil
	}); err != nil && ctx.Err() == nil {
		s.logger.Warn().Err(err).Msg("authz sweep lock failed")
	}
}

// Sweep runs one pass; tests call it directly.
func (s *Sweeper) Sweep(ctx context.Context) {
	s.replayMarks(ctx)
	if s.reconcileOverdue(ctx) {
		s.removeOrphanStores(ctx)
	}
}

func (s *Sweeper) replayMarks(ctx context.Context) {
	marks, err := s.repo.OldestInternal(ctx, sweepBatchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("list authz sync marks")
		return
	}
	refs := functional.Slice(marks).Map(func(m repository.Mark) OrganizationRef {
		return OrganizationRef{ProductID: m.ProductID, OrganizationID: m.OrganizationID}
	})
	syncMarked(ctx, s.syncer, refs, func(ref OrganizationRef, err error) {
		step := "replay organization"
		if ref.CoversProduct() {
			step = "replay product"
		}
		s.postponeAfterFailure(ctx, ref.ProductID, step, err)
	})
}

// postponeAfterFailure logs a failed step and postpones the Product's marks, so
// the next sweep reaches the other Products first.
func (s *Sweeper) postponeAfterFailure(ctx context.Context, productID, step string, err error) {
	if err == nil || ctx.Err() != nil {
		return
	}
	s.logger.Warn().Err(err).Str("product_id", productID).Str("step", step).
		Msg("authz sync failed; it is retried after the other products")
	if postponeErr := s.repo.PostponeInternal(ctx, productID, s.now().Add(failureBackoff)); postponeErr != nil {
		s.logger.Warn().Err(postponeErr).Str("product_id", productID).Msg("postpone authz marks")
	}
}

// reconcileOverdue rebuilds the Product rebuilt longest ago, if it is due, and
// reports whether it ran.
func (s *Sweeper) reconcileOverdue(ctx context.Context) bool {
	due, err := s.repo.DueForReconcileInternal(ctx, s.now().Add(-reconcileInterval))
	if err != nil {
		s.logger.Error().Err(err).Msg("find product due for authz reconcile")
		return false
	}
	productID, ok := due.Get()
	if !ok {
		return false
	}
	s.postponeAfterFailure(ctx, productID, "reconcile", s.syncer.Reconcile(ctx, productID))
	return true
}

// removeOrphanStores deletes, one page per reconcile, the stores whose Product
// is gone: a deleted Product's authz_stores row cascades away, but OpenFGA
// keeps its store. A store is named after its Product.
func (s *Sweeper) removeOrphanStores(ctx context.Context) {
	openfga, err := s.engines.Get()
	if err != nil {
		return
	}
	page, err := openfga.ListStores(ctx, &openfgav1.ListStoresRequest{
		PageSize: wrapperspb.Int32(readPage), ContinuationToken: s.storesToken,
	})
	if err != nil {
		s.logger.Warn().Err(err).Msg("list openfga stores")
		return
	}
	s.storesToken = page.GetContinuationToken()
	for _, found := range page.GetStores() {
		exists, existsErr := s.repo.ProductExists(ctx, found.GetName())
		if existsErr != nil || exists {
			continue
		}
		if _, err = openfga.DeleteStore(ctx, &openfgav1.DeleteStoreRequest{StoreId: found.GetId()}); err != nil {
			s.logger.Warn().Err(err).Str("store_id", found.GetId()).Msg("delete an orphan openfga store")
		}
	}
}

// Replayer replays a request's marks right after its transaction commits.
type Replayer struct {
	syncer Syncer
	logger zerolog.Logger
}

func NewReplayer(syncer Syncer, logger zerolog.Logger) *Replayer {
	return &Replayer{syncer: syncer, logger: logger.With().Str("component", "authz_replayer").Logger()}
}

// Replay syncs what the request marked. It runs detached from the request, so
// a client that hangs up does not cut a sync short; a failure leaves the mark
// for the sweeper.
func (r *Replayer) Replay(ctx context.Context, marked []OrganizationRef) {
	if len(marked) == 0 {
		return
	}
	replayCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replayTimeout)
	defer cancel()
	syncMarked(replayCtx, r.syncer, marked, r.logFailure)
}

func (r *Replayer) logFailure(ref OrganizationRef, err error) {
	r.logger.Warn().Err(err).Str("product_id", ref.ProductID).Str("organization_id", ref.OrganizationID).
		Msg("authz replay after commit failed; the sweeper retries it")
}

// syncMarked syncs each marked Product in the order it first appears: the
// whole Product when one of its refs covers it, else each marked Organization.
// onFailure gets the ref whose sync failed.
func syncMarked(ctx context.Context, syncer Syncer, refs []OrganizationRef, onFailure func(OrganizationRef, error)) {
	byProduct := map[string][]OrganizationRef{}
	var order []string
	for _, ref := range refs {
		if _, seen := byProduct[ref.ProductID]; !seen {
			order = append(order, ref.ProductID)
		}
		byProduct[ref.ProductID] = append(byProduct[ref.ProductID], ref)
	}
	for _, productID := range order {
		marked := functional.Slice(byProduct[productID])
		if whole, ok := marked.FindFirst(OrganizationRef.CoversProduct).Get(); ok {
			if err := syncer.SyncProduct(ctx, productID); err != nil {
				onFailure(whole, err)
			}
			continue
		}
		for _, ref := range marked {
			if err := syncer.SyncOrganization(ctx, ref.ProductID, ref.OrganizationID); err != nil {
				onFailure(ref, err)
			}
		}
	}
}
