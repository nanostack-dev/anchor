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

const copyBackfillLockKey = "organization_roles:copy-backfill"

type backfillParams struct {
	fx.In
	Lifecycle  fx.Lifecycle
	Lock       *pglock.Client
	Products   corerepository.ProductRepository
	Repo       repository.Repository
	Transactor transactor.Transactor
	Logger     zerolog.Logger
}

// RegisterCopyBackfill runs ADR-0022 D8 on every start: each template gets a
// key, each Organization a copy of each template, each membership that still
// holds a template moves to its Organization's copy, and every copy is
// brought in step with its template. It is idempotent, so rows the previous
// release writes during a rolling deploy are caught by the next start.
func RegisterCopyBackfill(p backfillParams) {
	logger := p.Logger.With().Str("component", "organization_role_backfill").Logger()
	p.Lifecycle.Append(fx.Hook{
		// The first run copies every template into every Organization, which
		// may outlast the start timeout, so it runs beside the start.
		OnStart: func(ctx context.Context) error {
			runCtx := context.WithoutCancel(ctx)
			go func() {
				acquired, err := p.Lock.TryWithLock(runCtx, copyBackfillLockKey,
					func(lockCtx context.Context, _ *sql.Tx) error {
						return BackfillCopies(lockCtx, p.Products, p.Repo, p.Transactor, logger)
					})
				if err != nil {
					logger.Error().Err(err).Msg("organization role backfill failed")
					return
				}
				if !acquired {
					logger.Debug().Msg("organization role backfill skipped: lock not acquired")
				}
			}()
			return nil
		},
	})
}

// BackfillCopies keys, copies, repoints and syncs every product, each step in
// its own transaction, so an interrupted run leaves no copy without its
// permissions. A product that fails is logged and the next one runs. Tests
// call it directly.
func BackfillCopies(
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
		if productErr := backfillProduct(ctx, repo, tx, prod.ID, logger); productErr != nil {
			logger.Error().
				Err(productErr).
				Str("product_id", prod.ID).
				Msg("organization role backfill failed for product")
		}
	}
	return nil
}

func backfillProduct(
	ctx context.Context, repo repository.Repository, tx transactor.Transactor, productID string, logger zerolog.Logger,
) error {
	var keyed, repointed int64
	if err := tx.InTx(ctx, func(txCtx context.Context) error {
		var keyErr error
		keyed, keyErr = repo.BackfillTemplateKeysInternal(txCtx, productID)
		return keyErr
	}); err != nil {
		return err
	}
	copied, err := copyAll(ctx, repo, tx, productID)
	if err != nil {
		return err
	}
	if err = tx.InTx(ctx, func(txCtx context.Context) error {
		var repointErr error
		repointed, repointErr = repo.RepointMemberships(txCtx, productID, "")
		return repointErr
	}); err != nil {
		return err
	}
	templateIDs, err := repo.TemplateIDsInternal(ctx, productID)
	if err != nil {
		return err
	}
	for _, templateID := range templateIDs {
		if err = tx.InTx(ctx, func(txCtx context.Context) error {
			return repo.SyncCopies(txCtx, productID, templateID)
		}); err != nil {
			return err
		}
	}
	if keyed > 0 || copied > 0 || repointed > 0 {
		logger.Info().Str("product_id", productID).Int64("keyed", keyed).Int("copied", copied).
			Int64("repointed", repointed).Msg("moved roles onto organization copies")
	}
	return nil
}

// copyAll copies every missing template into every Organization of the
// product, a batch per transaction, until none is missing or a batch copies
// nothing because every pair left is skipped.
func copyAll(ctx context.Context, repo repository.Repository, tx transactor.Transactor, productID string) (int, error) {
	copied := 0
	for {
		var pairs, inserted int
		err := tx.InTx(ctx, func(txCtx context.Context) error {
			missing, missingErr := repo.MissingCopies(txCtx, productID, "", "", copyBatch)
			if missingErr != nil {
				return missingErr
			}
			pairs = len(missing)
			var insertErr error
			inserted, insertErr = repo.InsertCopies(txCtx, productID, missing)
			copied += inserted
			return insertErr
		})
		if err != nil || pairs == 0 || inserted == 0 {
			return copied, err
		}
	}
}
