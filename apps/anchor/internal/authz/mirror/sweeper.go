package mirror

import (
	"context"
	"slices"
	"time"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"go.uber.org/fx"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	sweepInterval     = 5 * time.Second
	sweepBatchSize    = 100
	reconcileInterval = 15 * time.Minute
	// failureBackoff is how long a Product whose sync failed waits behind the
	// others before the sweeper tries it again.
	failureBackoff = time.Minute
	// replayTimeout bounds the replay that follows a request's commit; a model
	// change replays its whole Product.
	replayTimeout = 30 * time.Second
	storesPage    = 100
)

// timing holds the waits a test may shorten.
type timing struct {
	lockWait     time.Duration
	lockRetryMin time.Duration
	lockRetryMax time.Duration
}

func defaultTiming() timing {
	return timing{lockWait: lockWait, lockRetryMin: lockRetryMin, lockRetryMax: lockRetryMax}
}

// Replay syncs what a request marked, right after its transaction committed. It
// runs detached from the request, so a client that hangs up does not cut a sync
// short. A failure is logged and leaves the mark for the sweeper.
func (m *Mirror) Replay(ctx context.Context, marked []Scope) {
	if len(marked) == 0 {
		return
	}
	replayCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), replayTimeout)
	defer cancel()
	for _, scope := range smallestCover(marked) {
		if err := m.Sync(replayCtx, scope); err != nil {
			m.logger.Warn().Err(err).Str("product_id", scope.ProductID).Str("projection", scope.Projection).
				Str("key", scope.Key).Msg("authz replay after commit failed; the sweeper retries it")
		}
	}
}

// Sweep runs one pass: it syncs the oldest marks, then rebuilds the Product
// rebuilt longest ago if it is due, then removes one page of orphan stores.
func (m *Mirror) Sweep(ctx context.Context) {
	m.syncOldestMarks(ctx)
	if m.reconcileOverdue(ctx) {
		m.removeOrphanStores(ctx)
	}
}

// Run sweeps on one replica at a time until the application stops.
func (m *Mirror) Run(lc fx.Lifecycle) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(done)
				ticker := time.NewTicker(sweepInterval)
				defer ticker.Stop()
				for {
					m.sweepAlone(ctx)
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

func (m *Mirror) sweepAlone(ctx context.Context) {
	if _, err := m.locker.TrySweep(ctx, func(lockCtx context.Context) error {
		m.Sweep(lockCtx)
		return nil
	}); err != nil && ctx.Err() == nil {
		m.logger.Warn().Err(err).Msg("authz sweep lock failed")
	}
}

func (m *Mirror) syncOldestMarks(ctx context.Context) {
	marks, err := m.repo.Oldest(ctx, sweepBatchSize)
	if err != nil {
		m.logger.Error().Err(err).Msg("list authz sync marks")
		return
	}
	scopes := make([]Scope, 0, len(marks))
	for _, mark := range marks {
		scopes = append(scopes, mark.Scope)
	}
	for _, scope := range smallestCover(scopes) {
		m.postponeAfterFailure(ctx, scope.ProductID, "replay", m.Sync(ctx, scope))
	}
}

// postponeAfterFailure logs a failed step and postpones the Product's marks, so
// the next sweep reaches the other Products first.
func (m *Mirror) postponeAfterFailure(ctx context.Context, productID, step string, err error) {
	if err == nil || ctx.Err() != nil {
		return
	}
	m.logger.Warn().Err(err).Str("product_id", productID).Str("step", step).
		Msg("authz sync failed; it is retried after the other products")
	if postponeErr := m.repo.Postpone(ctx, productID, m.now().Add(failureBackoff)); postponeErr != nil {
		m.logger.Warn().Err(postponeErr).Str("product_id", productID).Msg("postpone authz marks")
	}
}

// reconcileOverdue rebuilds the Product rebuilt longest ago, if it is due, and
// reports whether it ran.
func (m *Mirror) reconcileOverdue(ctx context.Context) bool {
	due, err := m.repo.DueForReconcile(ctx, m.now().Add(-reconcileInterval))
	if err != nil {
		m.logger.Error().Err(err).Msg("find product due for authz reconcile")
		return false
	}
	productID, ok := due.Get()
	if !ok {
		return false
	}
	m.postponeAfterFailure(ctx, productID, "reconcile", m.Reconcile(ctx, productID))
	return true
}

// removeOrphanStores deletes, one page per call, the stores whose Product is
// gone: a deleted Product's records cascade away, but OpenFGA keeps its store.
// Stores names each store after its Product.
func (m *Mirror) removeOrphanStores(ctx context.Context) {
	engine, err := m.engines.Get()
	if err != nil {
		return
	}
	page, err := engine.ListStores(ctx, &openfgav1.ListStoresRequest{
		PageSize: wrapperspb.Int32(storesPage), ContinuationToken: m.storesToken,
	})
	if err != nil {
		m.logger.Warn().Err(err).Msg("list openfga stores")
		return
	}
	m.storesToken = page.GetContinuationToken()
	for _, found := range page.GetStores() {
		exists, existsErr := m.repo.ProductExists(ctx, found.GetName())
		if existsErr != nil || exists {
			continue
		}
		if _, err = engine.DeleteStore(ctx, &openfgav1.DeleteStoreRequest{StoreId: found.GetId()}); err != nil {
			m.logger.Warn().Err(err).Str("store_id", found.GetId()).Msg("delete an orphan openfga store")
		}
	}
}

// smallestCover drops every scope another one covers, keeping the order in
// which each first appears: a whole Product covers its projections, and every
// key of a projection covers each key.
func smallestCover(scopes []Scope) []Scope {
	var cover []Scope
	for _, scope := range scopes {
		if slices.ContainsFunc(cover, func(kept Scope) bool { return kept.covers(scope) }) {
			continue
		}
		cover = slices.DeleteFunc(cover, scope.covers)
		cover = append(cover, scope)
	}
	return cover
}
