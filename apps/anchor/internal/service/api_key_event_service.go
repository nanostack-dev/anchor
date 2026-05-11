package service

import (
	"context"

	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/repository"

	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/rs/zerolog"
)

type OrganizationAPIKeyEventService interface {
	ProcessQueueJob(ctx context.Context, job pgqueue.Job) error
}

type organizationAPIKeyEventService struct {
	organizationAPIKeyRepo repository.OrganizationAPIKeyRepository
	logger                 zerolog.Logger
}

func NewOrganizationAPIKeyEventService(
	organizationAPIKeyRepo repository.OrganizationAPIKeyRepository,
	logger zerolog.Logger,
) OrganizationAPIKeyEventService {
	return &organizationAPIKeyEventService{
		organizationAPIKeyRepo: organizationAPIKeyRepo,
		logger:                 logger.With().Str("component", "api_key_event_service").Logger(),
	}
}

func (s *organizationAPIKeyEventService) processOrganizationAPIKeyExpiration(
	ctx context.Context,
	payload organizationAPIKeyEventPayload,
) error {
	apiKey, err := s.organizationAPIKeyRepo.GetByIDInternal(ctx, payload.APIKeyID, nil)
	if err != nil {
		return err
	}
	if apiKey == nil {
		return nil
	}
	if apiKey.Status != orgapikey.StatusActive {
		return nil
	}
	if apiKey.ExpiresAt == nil || apiKey.ExpiresAt.After(nowUTC()) {
		return nil
	}

	return s.organizationAPIKeyRepo.UpdateStatus(
		ctx,
		payload.OrganizationID,
		payload.APIKeyID,
		orgapikey.StatusInactive,
		nil,
	)
}
