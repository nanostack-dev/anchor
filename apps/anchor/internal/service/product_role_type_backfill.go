package service

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/repository"
)

const roleTypeBackfillLockKey = "product_roles:type-backfill"

type roleTypeBackfillParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Lock      *pglock.Client
	Products  repository.ProductRepository
	Roles     repository.ProductRoleRepository
	Logger    zerolog.Logger
}

// RegisterRoleTypeBackfill stores MANAGED on every role a release before role
// types wrote, on every start. It is idempotent, so a role the previous
// release writes during a rolling deploy gets its type on the next start.
func RegisterRoleTypeBackfill(p roleTypeBackfillParams) {
	logger := p.Logger.With().Str("component", "role_type_backfill").Logger()
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			acquired, err := p.Lock.TryWithLock(ctx, roleTypeBackfillLockKey,
				func(lockCtx context.Context, _ *sql.Tx) error {
					return BackfillRoleTypes(lockCtx, p.Products, p.Roles, logger)
				})
			if err != nil {
				logger.Error().Err(err).Msg("role type backfill failed")
				return nil
			}
			if !acquired {
				logger.Debug().Msg("role type backfill skipped: lock not acquired")
			}
			return nil
		},
	})
}

// BackfillRoleTypes types every product's roles, one statement per product;
// tests call it directly.
func BackfillRoleTypes(
	ctx context.Context,
	products repository.ProductRepository,
	roles repository.ProductRoleRepository,
	logger zerolog.Logger,
) error {
	all, err := products.FindAllInternal(ctx)
	if err != nil {
		return err
	}
	for _, prod := range all {
		typed, typeErr := roles.BackfillTypesInternal(ctx, prod.ID)
		if typeErr != nil {
			return typeErr
		}
		if typed > 0 {
			logger.Info().Str("product_id", prod.ID).Int64("typed", typed).Msg("stored the type of managed roles")
		}
	}
	return nil
}
