package service

import (
	"context"
	"time"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
	"google.golang.org/protobuf/types/known/wrapperspb"

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
	// staleMarkAge is how late a replay is logged as authz.mark_stale: until
	// it runs, decisions see the access from before the write.
	staleMarkAge = time.Minute
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
	s.reconcileOverdue(ctx)
}

func (s *Sweeper) replayMarks(ctx context.Context) {
	marks, err := s.repo.OldestInternal(ctx, sweepBatchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("list authz sync marks")
		return
	}
	byProduct := map[string][]repository.Mark{}
	var order []string
	for _, mark := range marks {
		if age := s.now().Sub(mark.MarkedAt); age > staleMarkAge {
			s.logger.Warn().Str("event", "authz.mark_stale").Str("product_id", mark.ProductID).
				Str("organization_id", mark.OrganizationID).Dur("age", age).Msg("authz.mark_stale")
		}
		if _, seen := byProduct[mark.ProductID]; !seen {
			order = append(order, mark.ProductID)
		}
		byProduct[mark.ProductID] = append(byProduct[mark.ProductID], mark)
	}
	for _, productID := range order {
		if hasProductScope(byProduct[productID]) {
			s.report(ctx, productID, "replay product", s.syncer.SyncProduct(ctx, productID))
			continue
		}
		for _, mark := range byProduct[productID] {
			s.report(ctx, productID, "replay organization",
				s.syncer.SyncOrganization(ctx, productID, mark.OrganizationID))
		}
	}
}

// report logs a failed step and postpones the Product's marks, so the next
// sweep reaches the other Products first.
func (s *Sweeper) report(ctx context.Context, productID, step string, err error) {
	if err == nil || ctx.Err() != nil {
		return
	}
	s.logger.Warn().Err(err).Str("product_id", productID).Str("step", step).
		Msg("authz sync failed; it is retried after the other products")
	if postponeErr := s.repo.PostponeInternal(ctx, productID, s.now().Add(failureBackoff)); postponeErr != nil {
		s.logger.Warn().Err(postponeErr).Str("product_id", productID).Msg("postpone authz marks")
	}
}

func hasProductScope(marks []repository.Mark) bool {
	for _, mark := range marks {
		if mark.OrganizationID == repository.ProductScope {
			return true
		}
	}
	return false
}

func (s *Sweeper) reconcileOverdue(ctx context.Context) {
	due, err := s.repo.DueForReconcileInternal(ctx, s.now().Add(-reconcileInterval))
	if err != nil {
		s.logger.Error().Err(err).Msg("find product due for authz reconcile")
		return
	}
	if productID, ok := due.Get(); ok {
		s.report(ctx, productID, "reconcile", s.syncer.Reconcile(ctx, productID))
		s.removeOrphanStores(ctx)
	}
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

// Replay syncs what the request marked: the whole Product after a catalog
// change, else each Organization. It runs detached from the request, so a
// client that hangs up does not cut a sync short; a failure leaves the mark
// for the sweeper.
func (r *Replayer) Replay(ctx context.Context, marked []OrganizationRef) {
	if len(marked) == 0 {
		return
	}
	replayCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replayTimeout)
	defer cancel()
	wholeProduct := map[string]bool{}
	for _, ref := range marked {
		if ref.OrganizationID == repository.ProductScope {
			wholeProduct[ref.ProductID] = true
		}
	}
	for productID := range wholeProduct {
		r.report(productID, "", r.syncer.SyncProduct(replayCtx, productID))
	}
	for _, ref := range marked {
		if !wholeProduct[ref.ProductID] {
			r.report(
				ref.ProductID,
				ref.OrganizationID,
				r.syncer.SyncOrganization(replayCtx, ref.ProductID, ref.OrganizationID),
			)
		}
	}
}

func (r *Replayer) report(productID, organizationID string, err error) {
	if err != nil {
		r.logger.Warn().Err(err).Str("product_id", productID).Str("organization_id", organizationID).
			Msg("authz replay after commit failed; the sweeper retries it")
	}
}
