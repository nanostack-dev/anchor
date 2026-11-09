package service

import (
	"context"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"anchor/internal/repository"
)

type TenantService interface {
	IsTenantInit(ctx context.Context) (bool, error)
}

type tenantService struct {
	tenantRepo repository.TenantRepository
	logger     *slog.Logger
}

func NewTenantService(
	tenantRepo repository.TenantRepository,
	logger *slog.Logger,
) TenantService {
	return &tenantService{
		tenantRepo: tenantRepo,
		logger:     logger.With(slog.String("component", "tenant_service")),
	}
}

func (t *tenantService) IsTenantInit(ctx context.Context) (bool, error) {
	logger := t.logger.With(slog.String("operation", "IsTenantInit"))

	count, err := t.tenantRepo.Count(ctx)
	if err != nil {
		log.ErrorTo(ctx, logger, err, "failed to count tenants")
		return false, err
	}
	return count > 0, nil
}
