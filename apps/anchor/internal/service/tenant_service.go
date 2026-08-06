package service

import (
	"context"

	"anchor/internal/repository"

	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	"github.com/rs/zerolog"
)

type TenantService interface {
	IsTenantInit(ctx context.Context) (bool, error)
}

type tenantService struct {
	tenantRepo repository.TenantRepository
	logger     zerolog.Logger
}

func NewTenantService(
	tenantRepo repository.TenantRepository,
	logger zerolog.Logger,
) TenantService {
	return &tenantService{
		tenantRepo: tenantRepo,
		logger:     logger.With().Str("component", "tenant_service").Logger(),
	}
}

func (t *tenantService) IsTenantInit(ctx context.Context) (bool, error) {
	logger := t.logger.With().Str("operation", "IsTenantInit").Logger()

	count, err := t.tenantRepo.Count(ctx)
	if err != nil {
		log.Event(&logger, err).Msg("failed to count tenants")
		return false, err
	}
	return count > 0, nil
}
