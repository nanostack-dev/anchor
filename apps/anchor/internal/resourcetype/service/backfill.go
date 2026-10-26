package service

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/domain/resourcetype"
	corerepository "anchor/internal/repository"
	"anchor/internal/resourcetype/repository"
)

const organizationBackfillLockKey = "resource_types:organization-backfill"

type backfillParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Lock      *pglock.Client
	Products  corerepository.ProductRepository
	Repo      repository.Repository
	Logger    zerolog.Logger
}

// RegisterOrganizationBackfill runs ADR-0022 G1 on every start: each product
// gets its organization type, and its unscoped roles and permissions are
// scoped to it. It is idempotent, so a release that still wrote unscoped rows
// during a rolling deploy is caught by the next start.
func RegisterOrganizationBackfill(p backfillParams) {
	logger := p.Logger.With().Str("component", "resource_type_backfill").Logger()
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			acquired, err := p.Lock.TryWithLock(ctx, organizationBackfillLockKey,
				func(lockCtx context.Context, _ *sql.Tx) error {
					return BackfillOrganizations(lockCtx, p.Products, p.Repo, logger)
				})
			if err != nil {
				logger.Error().Err(err).Msg("resource type backfill failed")
				return nil
			}
			if !acquired {
				logger.Debug().Msg("resource type backfill skipped: lock not acquired")
			}
			return nil
		},
	})
}

// BackfillOrganizations seeds and scopes every product; tests call it directly.
func BackfillOrganizations(
	ctx context.Context, products corerepository.ProductRepository, repo repository.Repository, logger zerolog.Logger,
) error {
	all, err := products.FindAllInternal(ctx)
	if err != nil {
		return err
	}
	for _, prod := range all {
		if err = repo.EnsureOrganization(ctx, prod.ID); err != nil {
			return err
		}
		var types []resourcetype.ResourceType
		if types, err = repo.FindTree(ctx, prod.ID); err != nil {
			return err
		}
		root := ""
		for _, rt := range types {
			if rt.BuiltIn() {
				root = rt.ID
			}
		}
		var scoped int64
		if scoped, err = repo.BackfillScopesInternal(ctx, prod.ID, root); err != nil {
			return err
		}
		if scoped > 0 {
			logger.Info().Str("product_id", prod.ID).Int64("scoped", scoped).
				Msg("scoped roles and permissions to the organization type")
		}
	}
	return nil
}
