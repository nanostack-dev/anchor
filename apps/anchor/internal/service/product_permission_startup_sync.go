package service

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/pgkit/pglock"
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
	Logger                *slog.Logger
}

func RegisterProductPermissionStartupSync(p productPermissionStartupSyncParams) {
	logger := p.Logger.With(slog.String("component", "product_permission_startup_sync"))

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
				logger.ErrorContext(ctx, "startup product permission sync failed", log.Err(err))
				return nil
			}
			if !acquired {
				logger.DebugContext(ctx, "startup product permission sync skipped: lock not acquired")
			}

			return nil
		},
	})
}

func syncProductPermissions(
	ctx context.Context,
	prod product.Product,
	permRepo repository.ProductPermissionRepository,
	logger *slog.Logger,
) error {
	desired := functional.Slice(GeneratePermissions()).
		Map(func(perm permission.ProductPermission) permission.ProductPermission {
			perm.ProductID = prod.ID
			return perm
		}).
		ToMap(func(perm permission.ProductPermission) string { return perm.Name })

	existing, err := listAllProductPermissions(ctx, permRepo, prod.ID)
	if err != nil {
		return err
	}

	existingByName := functional.Slice(existing).ToMap(func(perm permission.ProductPermission) string {
		return perm.Name
	})

	for name, perm := range desired {
		if _, ok := existingByName[name]; ok {
			continue
		}
		_, createErr := permRepo.Create(ctx, perm)
		if createErr != nil {
			return createErr
		}
		logger.InfoContext(ctx, "product permission inserted during startup sync",
			slog.String("product_id", prod.ID),
			slog.String("permission", name),
		)
	}

	for name := range existingByName {
		if _, ok := desired[name]; ok {
			continue
		}
		deleteErr := permRepo.DeleteByID(ctx, prod.ID, name)
		if deleteErr != nil {
			return deleteErr
		}
		logger.InfoContext(ctx, "product permission deleted during startup sync",
			slog.String("product_id", prod.ID),
			slog.String("permission", name),
		)
	}

	return nil
}

func listAllProductPermissions(
	ctx context.Context,
	permRepo repository.ProductPermissionRepository,
	productID string,
) ([]permission.ProductPermission, error) {
	const pageSize = int32(200)
	var all []permission.ProductPermission

	request := search.Request[
		permission.SearchProductPermissionFilter,
		permission.SortFieldProductPermission,
	]{
		Pagination: search.Pagination{Limit: pageSize},
	}

	for {
		result, err := permRepo.SearchByProduct(ctx, productID, request)
		if err != nil {
			return nil, err
		}

		all = append(all, result.Items...)
		if result.NextCursor == "" {
			break
		}
		request = request.WithCursor(result.NextCursor)
	}

	return all, nil
}
