package service

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/authz/repository"
)

const (
	sweepInterval     = 5 * time.Second
	sweepBatchSize    = 100
	reconcileInterval = 15 * time.Minute
)

// Sweeper keeps every store converging without a decision asking for it:
//   - it replays marks no decision has read, including product-scope marks,
//     which decisions never wait for;
//   - it creates and fills the store of every product that has none;
//   - it rewrites one product at a time in full once its last rewrite is older
//     than reconcileInterval, repairing drift from writes that left no mark.
//
// Its state lives in Postgres (marks, authz_stores), so a restart loses
// nothing, and the product lock keeps replicas from repeating each other.
type Sweeper struct {
	syncer *Syncer
	repo   repository.Repository
	logger zerolog.Logger
	now    func() time.Time
}

func NewSweeper(syncer *Syncer, repo repository.Repository, logger zerolog.Logger) *Sweeper {
	return &Sweeper{
		syncer: syncer,
		repo:   repo,
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
					s.sweep(ctx)
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

func (s *Sweeper) sweep(ctx context.Context) {
	s.fillMissingStores(ctx)
	s.replayMarks(ctx)
	s.reconcileOverdue(ctx)
}

func (s *Sweeper) fillMissingStores(ctx context.Context) {
	productIDs, err := s.repo.ProductIDsWithoutStoreInternal(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("list products without an authz store")
		return
	}
	for _, productID := range productIDs {
		s.report(ctx, productID, "fill store", s.syncer.SyncProduct(ctx, productID))
	}
}

func (s *Sweeper) replayMarks(ctx context.Context) {
	marks, err := s.repo.OldestInternal(ctx, sweepBatchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("list authz sync marks")
		return
	}
	seen := map[string]bool{}
	for _, mark := range marks {
		if !seen[mark.ProductID] {
			seen[mark.ProductID] = true
			s.report(ctx, mark.ProductID, "replay marks", s.syncer.SyncProduct(ctx, mark.ProductID))
		}
	}
}

func (s *Sweeper) reconcileOverdue(ctx context.Context) {
	due, err := s.repo.DueForReconcileInternal(ctx, s.now().Add(-reconcileInterval))
	if err != nil {
		s.logger.Error().Err(err).Msg("find product due for authz reconcile")
		return
	}
	if productID, ok := due.Get(); ok {
		s.report(ctx, productID, "reconcile", s.syncer.Reconcile(ctx, productID))
	}
}

func (s *Sweeper) report(ctx context.Context, productID, step string, err error) {
	if err != nil && ctx.Err() == nil {
		s.logger.Error().Err(err).Str("product_id", productID).Str("step", step).
			Msg("authz sync failed; it is retried on the next sweep")
	}
}
