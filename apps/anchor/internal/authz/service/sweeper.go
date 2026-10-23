package service

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/authz/repository"
)

const (
	sweepInterval  = 5 * time.Second
	sweepBatchSize = 100
)

// Sweeper replays marks nobody has read yet, so the relationship store
// converges even for organizations no decision is asked about. Marks live in
// Postgres, so a restart loses nothing: the next sweep picks them up.
type Sweeper struct {
	syncer *Syncer
	repo   repository.Repository
	logger zerolog.Logger
}

func NewSweeper(syncer *Syncer, repo repository.Repository, logger zerolog.Logger) *Sweeper {
	return &Sweeper{syncer: syncer, repo: repo, logger: logger.With().Str("component", "authz_sweeper").Logger()}
}

// Run sweeps until the application stops. The first sweep also creates and
// fills the store of every product that has none, such as on the first start
// after this feature ships.
func (s *Sweeper) Run(lc fx.Lifecycle) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(done)
				s.backfill(ctx)
				ticker := time.NewTicker(sweepInterval)
				defer ticker.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						s.sweep(ctx)
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

func (s *Sweeper) backfill(ctx context.Context) {
	productIDs, err := s.repo.ProductIDsInternal(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("list products for authz backfill")
		return
	}
	for _, productID := range productIDs {
		if ctx.Err() != nil {
			return
		}
		store, findErr := s.repo.FindStoreID(ctx, productID)
		if findErr != nil || store.IsPresent() {
			continue
		}
		s.syncProduct(ctx, productID)
	}
}

func (s *Sweeper) sweep(ctx context.Context) {
	marks, err := s.repo.OldestInternal(ctx, sweepBatchSize)
	if err != nil {
		s.logger.Error().Err(err).Msg("list authz sync marks")
		return
	}
	seen := map[string]bool{}
	for _, mark := range marks {
		if ctx.Err() != nil {
			return
		}
		if !seen[mark.ProductID] {
			seen[mark.ProductID] = true
			s.syncProduct(ctx, mark.ProductID)
		}
	}
}

func (s *Sweeper) syncProduct(ctx context.Context, productID string) {
	if err := s.syncer.SyncProduct(ctx, productID); err != nil && ctx.Err() == nil {
		s.logger.Error().
			Err(err).
			Str("product_id", productID).
			Msg("authz sync failed; the mark stays for the next sweep")
	}
}
