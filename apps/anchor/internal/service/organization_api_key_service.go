package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"anchor/internal/security"

	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

	orgapikey "anchor/internal/domain/organization/apikey"
	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
)

type OrganizationAPIKeyService interface {
	Create(
		ctx context.Context,
		input orgapikey.CreateOrganizationAPIKeyInput,
	) (orgapikey.OrganizationAPIKey, string, error)
	GetByID(
		ctx context.Context,
		input orgapikey.GetOrganizationAPIKeyInput,
	) (*orgapikey.OrganizationAPIKey, error)
	Update(
		ctx context.Context,
		input orgapikey.UpdateOrganizationAPIKeyInput,
	) (orgapikey.OrganizationAPIKey, error)
	Search(
		ctx context.Context,
		input orgapikey.SearchOrganizationAPIKeysInput,
	) (*search.Result[orgapikey.OrganizationAPIKey], error)
	Delete(
		ctx context.Context,
		input orgapikey.DeleteOrganizationAPIKeyInput,
	) error
	ValidateAPIKeyAndScopes(
		ctx context.Context,
		input orgapikey.ValidateOrganizationAPIKeyScopesInput,
	) (orgapikey.ValidateOrganizationAPIKeyScopesOutput, error)
}

type organizationAPIKeyService struct {
	db               *sql.DB
	queue            *pgqueue.Client
	apiKeyRepo       repository.OrganizationAPIKeyRepository
	organizationRepo repository.OrganizationRepository
	permissionRepo   repository.ProductResourcePermissionRepository
	logger           zerolog.Logger
}

func NewOrganizationAPIKeyService(
	db *sql.DB,
	queue *pgqueue.Client,
	apiKeyRepo repository.OrganizationAPIKeyRepository,
	organizationRepo repository.OrganizationRepository,
	permissionRepo repository.ProductResourcePermissionRepository,
	logger zerolog.Logger,
) OrganizationAPIKeyService {
	return &organizationAPIKeyService{
		db:               db,
		queue:            queue,
		apiKeyRepo:       apiKeyRepo,
		organizationRepo: organizationRepo,
		permissionRepo:   permissionRepo,
		logger: logger.With().Str(
			"component", "organization_api_key_service",
		).Logger(),
	}
}

func (s *organizationAPIKeyService) Create(
	ctx context.Context,
	input orgapikey.CreateOrganizationAPIKeyInput,
) (orgapikey.OrganizationAPIKey, string, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return orgapikey.OrganizationAPIKey{}, "", err
	}

	if input.ExpiresAt != nil && !input.ExpiresAt.After(nowUTC().Truncate(time.Second)) {
		return orgapikey.OrganizationAPIKey{}, "", NewOrganizationAPIKeyExpiresAtInPastError()
	}

	org, err := s.organizationRepo.FindByID(ctx, input.ProductID, input.OrganizationID, nil)
	if err != nil {
		return orgapikey.OrganizationAPIKey{}, "", toolkit.ErrUnexpected
	}
	if org == nil {
		return orgapikey.OrganizationAPIKey{}, "", toolkit.ErrNotFound
	}

	if nameValidationErr := s.nameUniqueValidation(
		ctx,
		input.OrganizationID,
		input.Name,
		logger,
	); nameValidationErr != nil {
		return orgapikey.OrganizationAPIKey{}, "", nameValidationErr
	}

	clearAPIKey, err := security.GenerateOrganizationAPIKey()
	if err != nil {
		logger.Error().Err(err).Msg("failed to generate organization API key")
		return orgapikey.OrganizationAPIKey{}, "", toolkit.ErrUnexpected
	}

	hashedValue := security.HashSecret(clearAPIKey)
	obfuscatedValue := security.ObfuscateOrganizationAPIKey(clearAPIKey)

	organizationAPIKey := orgapikey.OrganizationAPIKey{
		OrganizationID:  input.OrganizationID,
		Name:            input.Name,
		Description:     input.Description,
		HashedValue:     hashedValue,
		ObfuscatedValue: obfuscatedValue,
		Status:          orgapikey.StatusActive,
		ExpiresAt:       input.ExpiresAt,
	}
	organizationAPIKey.GenerateID()

	if permissionValidationErr := s.permissionsValidation(
		ctx,
		org.ProductID,
		input.Permissions,
		logger,
	); permissionValidationErr != nil {
		return orgapikey.OrganizationAPIKey{}, "", permissionValidationErr
	}

	organizationAPIKey.Permissions = toolkit.TransformSlice(
		input.Permissions,
		func(perm string) orgapikey.OrganizationAPIKeyPermission {
			return orgapikey.OrganizationAPIKeyPermission{
				APIKeyID:       organizationAPIKey.ID,
				OrganizationID: input.OrganizationID,
				ProductID:      org.ProductID,
				PermissionName: perm,
				CreatedAt:      time.Now(),
			}
		},
	)

	created, err := toolkit.WithTxReturn(s.db, func(tx *sql.Tx) (orgapikey.OrganizationAPIKey, error) {
		txOptions := &toolkit.DBOptions{Tx: tx}

		created, createErr := s.apiKeyRepo.Create(ctx, organizationAPIKey, txOptions)
		if createErr != nil {
			return orgapikey.OrganizationAPIKey{}, createErr
		}

		if enqueueErr := s.enqueueExpirationEvent(ctx, tx, created); enqueueErr != nil {
			return orgapikey.OrganizationAPIKey{}, enqueueErr
		}

		return created, nil
	})
	if err != nil {
		logger.Error().
			Str("organization_api_key_id", organizationAPIKey.ID).
			Str("organization_id", input.OrganizationID).
			Err(err).
			Msg("failed to create organization API key")
		return orgapikey.OrganizationAPIKey{}, "", toolkit.ErrUnexpected
	}

	return created, clearAPIKey, nil
}

func (s *organizationAPIKeyService) GetByID(
	ctx context.Context,
	input orgapikey.GetOrganizationAPIKeyInput,
) (*orgapikey.OrganizationAPIKey, error) {
	logger := s.logger.With().Str("operation", "GetByID").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}
	if err := s.ensureOrganizationBelongsToProduct(ctx, input.ProductID, input.OrganizationID); err != nil {
		return nil, err
	}

	apiKey, err := s.apiKeyRepo.GetByID(ctx, input.OrganizationID, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get organization API key")
		return nil, toolkit.ErrUnexpected
	}

	if apiKey == nil {
		return nil, toolkit.ErrNotFound
	}

	return apiKey, nil
}

func (s *organizationAPIKeyService) Update(
	ctx context.Context,
	input orgapikey.UpdateOrganizationAPIKeyInput,
) (orgapikey.OrganizationAPIKey, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return orgapikey.OrganizationAPIKey{}, err
	}
	if err := s.ensureOrganizationBelongsToProduct(ctx, input.ProductID, input.OrganizationID); err != nil {
		return orgapikey.OrganizationAPIKey{}, err
	}

	existingAPIKey, err := s.apiKeyRepo.GetByID(ctx, input.OrganizationID, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get organization API key for update")
		return orgapikey.OrganizationAPIKey{}, toolkit.ErrUnexpected
	}

	if existingAPIKey == nil {
		return orgapikey.OrganizationAPIKey{}, toolkit.ErrNotFound
	}

	updatedAPIKey := *existingAPIKey
	if input.Name != nil && *input.Name != updatedAPIKey.Name {
		if nameValidationErr := s.nameUniqueValidation(
			ctx,
			input.OrganizationID,
			*input.Name,
			logger,
		); nameValidationErr != nil {
			return orgapikey.OrganizationAPIKey{}, nameValidationErr
		}
		updatedAPIKey.Name = *input.Name
	}
	if input.Description != nil {
		updatedAPIKey.Description = input.Description
	}
	if input.Status != nil {
		if *input.Status == orgapikey.StatusActive && existingAPIKey.IsExpiredAt(nowUTC()) {
			return orgapikey.OrganizationAPIKey{}, NewOrganizationAPIKeyInactiveOrExpiredError(input.ID)
		}
		updatedAPIKey.Status = *input.Status
	}

	updated, err := s.apiKeyRepo.Update(ctx, updatedAPIKey, nil)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to update organization API key")
		return orgapikey.OrganizationAPIKey{}, toolkit.ErrUnexpected
	}

	return updated, nil
}

func (s *organizationAPIKeyService) Search(
	ctx context.Context,
	input orgapikey.SearchOrganizationAPIKeysInput,
) (*search.Result[orgapikey.OrganizationAPIKey], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}
	if err := s.ensureOrganizationBelongsToProduct(ctx, input.ProductID, input.OrganizationID); err != nil {
		return nil, err
	}

	result, err := s.apiKeyRepo.SearchByOrganizationID(ctx, input, nil)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Err(err).
			Msg("failed to search organization API keys")
		return nil, toolkit.ErrUnexpected
	}

	return &result, nil
}

func (s *organizationAPIKeyService) Delete(
	ctx context.Context,
	input orgapikey.DeleteOrganizationAPIKeyInput,
) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return err
	}
	if err := s.ensureOrganizationBelongsToProduct(ctx, input.ProductID, input.OrganizationID); err != nil {
		return err
	}

	existingAPIKey, err := s.apiKeyRepo.GetByID(ctx, input.OrganizationID, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get organization API key for deletion")
		return toolkit.ErrUnexpected
	}

	if existingAPIKey == nil {
		return toolkit.ErrNotFound
	}

	return toolkit.WithTx(s.db, func(tx *sql.Tx) error {
		txOpts := &toolkit.DBOptions{Tx: tx}

		if deleteErr := s.apiKeyRepo.Delete(ctx, input.OrganizationID, input.ID, txOpts); deleteErr != nil {
			logger.Error().
				Str("organization_id", input.OrganizationID).
				Str("api_key_id", input.ID).
				Err(deleteErr).
				Msg("failed to delete organization API key")
			return toolkit.ErrUnexpected
		}

		if existingAPIKey.ExpiresAt == nil {
			return nil
		}

		jobs, listErr := s.queue.ListJobs(ctx, pgqueue.ListJobsParams{
			QueueName: organizationAPIKeyEventQueueName,
			Status:    pgqueue.StatusPending,
			Limit:     organizationAPIKeyEventListLimit,
		})
		if listErr != nil {
			logger.Error().
				Str("api_key_id", input.ID).
				Err(listErr).
				Msg("failed to list pending queue jobs for api key deletion")
			return toolkit.ErrUnexpected
		}

		for _, job := range jobs {
			var p organizationAPIKeyEventPayload
			if unmarshalErr := json.Unmarshal(job.Payload, &p); unmarshalErr != nil {
				continue
			}
			if p.APIKeyID != input.ID {
				continue
			}
			if cancelErr := s.queue.DeleteJob(ctx, job.ID); cancelErr != nil {
				logger.Error().
					Str("api_key_id", input.ID).
					Int64("job_id", job.ID).
					Err(cancelErr).
					Msg("failed to cancel queue job for deleted api key")
				return toolkit.ErrUnexpected
			}
		}

		return nil
	})
}

func (s *organizationAPIKeyService) ValidateAPIKeyAndScopes(
	ctx context.Context,
	input orgapikey.ValidateOrganizationAPIKeyScopesInput,
) (orgapikey.ValidateOrganizationAPIKeyScopesOutput, error) {
	logger := s.logger.With().Str("operation", "ValidateAPIKeyAndScopes").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return orgapikey.ValidateOrganizationAPIKeyScopesOutput{}, err
	}
	if err := s.ensureOrganizationBelongsToProduct(ctx, input.ProductID, input.OrganizationID); err != nil {
		return orgapikey.ValidateOrganizationAPIKeyScopesOutput{}, err
	}

	validAPIKey, inactive, err := s.validateAPIKey(ctx, input.OrganizationID, input.APIKeyValue, logger)
	if err != nil {
		return orgapikey.ValidateOrganizationAPIKeyScopesOutput{}, err
	}

	permissionMap := make(map[string]bool, len(validAPIKey.Permissions))
	currentScopes := make([]string, 0, len(validAPIKey.Permissions))
	for _, perm := range validAPIKey.Permissions {
		permissionMap[perm.PermissionName] = true
		currentScopes = append(currentScopes, perm.PermissionName)
	}

	var missingScopes []string
	for _, scope := range input.Scopes {
		if !permissionMap[scope] {
			missingScopes = append(missingScopes, scope)
		}
	}

	output := orgapikey.ValidateOrganizationAPIKeyScopesOutput{
		APIKey:            validAPIKey,
		Permissions:       currentScopes,
		MissingPrivileges: missingScopes,
		Authorized:        len(missingScopes) == 0 && !inactive,
		Inactive:          inactive,
	}

	if inactive {
		logger.Debug().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", validAPIKey.ID).
			Msg("organization API key is inactive")
		return output, nil
	}

	if len(missingScopes) > 0 {
		logger.Debug().
			Str("organization_id", input.OrganizationID).
			Strs("missing_scopes", missingScopes).
			Msg("organization API key does not have required scopes")
		return output, nil
	}

	return output, nil
}

func (s *organizationAPIKeyService) validateAPIKey(
	ctx context.Context,
	organizationID string,
	apiKey string,
	logger zerolog.Logger,
) (orgapikey.OrganizationAPIKey, bool, error) {
	if apiKey == "" {
		return orgapikey.OrganizationAPIKey{}, false, ErrInvalidAPIKey
	}

	hashedKey := security.HashSecret(apiKey)
	foundAPIKey, err := s.apiKeyRepo.GetByOrganizationIDAndHashedValue(ctx, organizationID, hashedKey, nil)
	if err != nil {
		logger.Error().Str("organization_id", organizationID).Err(err).Msg("failed to validate organization API key")
		return orgapikey.OrganizationAPIKey{}, false, toolkit.ErrUnexpected
	}
	if foundAPIKey == nil {
		return orgapikey.OrganizationAPIKey{}, false, ErrInvalidAPIKey
	}

	now := nowUTC()
	if foundAPIKey.IsExpiredAt(now) {
		if foundAPIKey.Status == orgapikey.StatusActive {
			if updateErr := s.apiKeyRepo.UpdateStatus(
				ctx,
				foundAPIKey.OrganizationID,
				foundAPIKey.ID,
				orgapikey.StatusInactive,
				nil,
			); updateErr != nil {
				logger.Error().
					Str("organization_id", organizationID).
					Str("api_key_id", foundAPIKey.ID).
					Err(updateErr).
					Msg("failed to update expired organization API key status")
			} else {
				foundAPIKey.Status = orgapikey.StatusInactive
			}
		}
		return *foundAPIKey, true, nil
	}
	if foundAPIKey.Status != orgapikey.StatusActive {
		return *foundAPIKey, true, nil
	}

	shouldUpdate := foundAPIKey.LastUsedAt == nil || time.Since(*foundAPIKey.LastUsedAt) > time.Hour
	if shouldUpdate {
		if updateErr := s.apiKeyRepo.UpdateLastUsedAt(
			ctx,
			foundAPIKey.OrganizationID,
			foundAPIKey.ID,
			nil,
		); updateErr != nil {
			logger.Error().
				Str("organization_id", organizationID).
				Str("api_key_id", foundAPIKey.ID).
				Err(updateErr).
				Msg("failed to update organization API key last used timestamp")
		} else {
			foundAPIKey.LastUsedAt = &now
		}
	}

	return *foundAPIKey, false, nil
}

func (s *organizationAPIKeyService) enqueueExpirationEvent(
	ctx context.Context,
	tx *sql.Tx,
	apiKey orgapikey.OrganizationAPIKey,
) error {
	if apiKey.ExpiresAt == nil {
		return nil
	}

	payload, err := json.Marshal(organizationAPIKeyEventPayload{
		EventType:      organizationAPIKeyEventTypeExpiration,
		OrganizationID: apiKey.OrganizationID,
		APIKeyID:       apiKey.ID,
	})
	if err != nil {
		return err
	}

	_, err = s.queue.EnqueueTx(ctx, tx, pgqueue.EnqueueParams{
		QueueName:   organizationAPIKeyEventQueueName,
		Payload:     payload,
		AvailableAt: apiKey.ExpiresAt,
		MaxAttempts: organizationAPIKeyEventMaxAttempts,
	})

	return err
}

func (s *organizationAPIKeyService) nameUniqueValidation(
	ctx context.Context,
	organizationID, name string,
	logger zerolog.Logger,
) error {
	existingAPIKey, err := s.apiKeyRepo.GetByOrganizationIDAndName(ctx, organizationID, name, nil)
	if err != nil {
		logger.Error().
			Str("organization_id", organizationID).
			Str("name", name).
			Err(err).
			Msg("failed to search for organization API keys by name")
		return toolkit.ErrUnexpected
	}
	if existingAPIKey != nil {
		return NewOrganizationAPIKeyNameExistsError(name, organizationID)
	}
	return nil
}

func (s *organizationAPIKeyService) ensureOrganizationBelongsToProduct(
	ctx context.Context,
	productID, organizationID string,
) error {
	org, err := s.organizationRepo.FindByID(ctx, productID, organizationID, nil)
	if err != nil {
		return toolkit.ErrUnexpected
	}
	if org == nil {
		return toolkit.ErrNotFound
	}
	return nil
}

func (s *organizationAPIKeyService) permissionsValidation(
	ctx context.Context,
	productID string,
	permissionNames []string,
	logger zerolog.Logger,
) error {
	permsFound, err := s.permissionRepo.FindByProductIDAndPermissionNames(
		ctx, productID, permissionNames, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Int("permission_count", len(permissionNames)).
			Err(err).
			Msg("failed to find permissions by names")
		return err
	}
	if len(permsFound) != len(permissionNames) {
		foundNames := resourcePermissionsToStrings(permsFound)
		missingNames := toolkit.StringSliceDiff(permissionNames, foundNames)
		return NewPermissionsNotFoundError(productID, missingNames)
	}
	return nil
}

func resourcePermissionsToStrings(
	input []resourcepermission.ProductResourcePermission,
) []string {
	return toolkit.TransformSlice(
		input,
		func(permission resourcepermission.ProductResourcePermission) string {
			return permission.Name
		},
	)
}
