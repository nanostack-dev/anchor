package service

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/organizationrole/repository"
	corerepository "anchor/internal/repository"
)

const keyBackfillLockKey = "organization_roles:key-backfill"

type backfillParams struct {
	fx.In
	Lifecycle  fx.Lifecycle
	Lock       *pglock.Client
	Products   corerepository.ProductRepository
	Repo       repository.Repository
	Transactor transactor.Transactor
	Logger     zerolog.Logger
}

// RegisterKeyBackfill gives every role template a key on every start
// (ADR-0022 D8). It is idempotent, so a template a previous release writes
// during a rolling deploy gets its key on the next start.
func RegisterKeyBackfill(p backfillParams) {
	logger := p.Logger.With().Str("component", "organization_role_backfill").Logger()
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			acquired, err := p.Lock.TryWithLock(ctx, keyBackfillLockKey,
				func(lockCtx context.Context, _ *sql.Tx) error {
					return BackfillKeys(lockCtx, p.Products, p.Repo, p.Transactor, logger)
				})
			if err != nil {
				logger.Error().Err(err).Msg("role template key backfill failed")
				return nil
			}
			if !acquired {
				logger.Debug().Msg("role template key backfill skipped: lock not acquired")
			}
			return nil
		},
	})
}

// BackfillKeys keys every product's templates, one transaction per product;
// tests call it directly.
func BackfillKeys(
	ctx context.Context,
	products corerepository.ProductRepository,
	repo repository.Repository,
	tx transactor.Transactor,
	logger zerolog.Logger,
) error {
	all, err := products.FindAllInternal(ctx)
	if err != nil {
		return err
	}
	for _, prod := range all {
		var keyed int64
		if err = tx.InTx(ctx, func(txCtx context.Context) error {
			var keyErr error
			keyed, keyErr = repo.BackfillTemplateKeysInternal(txCtx, prod.ID)
			return keyErr
		}); err != nil {
			return err
		}
		if keyed > 0 {
			logger.Info().Str("product_id", prod.ID).Int64("keyed", keyed).Msg("gave role templates their keys")
		}
	}
	return nil
}
