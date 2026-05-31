package service

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/domain/permission"
	"anchor/internal/domain/product"
	"anchor/internal/repository"
)

const productPermissionStartupSyncLockKey = "product_permissions:startup-sync"

type productPermissionStartupSyncParams struct {
	fx.In
	Lifecycle             fx.Lifecycle
	Lock                  *pglock.Client
	ProductRepo           repository.ProductRepository
	ProductPermissionRepo repository.ProductPermissionRepository
	Logger                zerolog.Logger
}

func RegisterProductPermissionStartupSync(p productPermissionStartupSyncParams) {
	logger := p.Logger.With().Str("component", "product_permission_startup_sync").Logger()

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			acquired, err := p.Lock.TryWithLock(
				ctx,
				productPermissionStartupSyncLockKey,
				func(lockCtx context.Context, _ *sql.Tx) error {
					products, listErr := p.ProductRepo.FindAllInternal(lockCtx)
					if listErr != nil {
						return listErr
					}

					for _, prod := range products {
						syncErr := syncProductPermissions(
							lockCtx,
							prod,
							p.ProductPermissionRepo,
							logger,
						)
						if syncErr != nil {
							return syncErr
						}
					}

					return nil
				},
			)
			if err != nil {
				logger.Error().Err(err).Msg("startup product permission sync failed")
				return nil
			}
			if !acquired {
				logger.Debug().Msg("startup product permission sync skipped: lock not acquired")
			}

			return nil
		},
	})
}

func syncProductPermissions(
	ctx context.Context,
	prod product.Product,
	permRepo repository.ProductPermissionRepository,
	logger zerolog.Logger,
) error {
	desired := make(map[string]permission.ProductPermission)
	for _, perm := range GeneratePermissions() {
		perm.ProductID = prod.ID
		desired[perm.Name] = perm
	}

	existing, err := listAllProductPermissions(ctx, permRepo, prod.ID)
	if err != nil {
		return err
	}

	existingByName := make(map[string]permission.ProductPermission)
	for _, perm := range existing {
		existingByName[perm.Name] = perm
	}

	for name, perm := range desired {
		if _, ok := existingByName[name]; ok {
			continue
		}
		_, createErr := permRepo.Create(ctx, perm)
		if createErr != nil {
			return createErr
		}
		logger.Info().
			Str("product_id", prod.ID).
			Str("permission", name).
			Msg("product permission inserted during startup sync")
	}

	for name := range existingByName {
		if _, ok := desired[name]; ok {
			continue
		}
		deleteErr := permRepo.DeleteByID(ctx, prod.ID, name)
		if deleteErr != nil {
			return deleteErr
		}
		logger.Info().
			Str("product_id", prod.ID).
			Str("permission", name).
			Msg("product permission deleted during startup sync")
	}

	return nil
}

func listAllProductPermissions(
	ctx context.Context,
	permRepo repository.ProductPermissionRepository,
	productID string,
) ([]permission.ProductPermission, error) {
	const pageSize = int32(200)
	var (
		offset int32
		all    []permission.ProductPermission
	)

	for {
		request := search.Request[
			permission.SearchProductPermissionFilter,
			permission.SortFieldProductPermission,
		]{
			Pagination: search.Pagination{Limit: pageSize, Offset: offset},
		}

		result, err := permRepo.SearchByProduct(ctx, productID, request)
		if err != nil {
			return nil, err
		}

		all = append(all, result.Items...)
		if len(result.Items) < int(pageSize) {
			break
		}
		offset += pageSize
	}

	return all, nil
}
