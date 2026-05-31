package service

import (
	"context"
	"encoding/json"
	"time"

	"anchor/internal/security"

	apierror "github.com/nanostack-dev/nanostack-framework/pkg/apierror"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"
	"github.com/nanostack-dev/pgkit/pgqueue"

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
	transactor       transactor.Transactor
	queue            *pgqueue.Client
	apiKeyRepo       repository.OrganizationAPIKeyRepository
	organizationRepo repository.OrganizationRepository
	permissionRepo   repository.ProductResourcePermissionRepository
	logger           zerolog.Logger
}

func NewOrganizationAPIKeyService(
	transactor transactor.Transactor,
	queue *pgqueue.Client,
	apiKeyRepo repository.OrganizationAPIKeyRepository,
	organizationRepo repository.OrganizationRepository,
	permissionRepo repository.ProductResourcePermissionRepository,
	logger zerolog.Logger,
) OrganizationAPIKeyService {
	return &organizationAPIKeyService{
		transactor:       transactor,
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

	if err := validateStruct(input); err != nil {
		return orgapikey.OrganizationAPIKey{}, "", err
	}

	if input.ExpiresAt != nil && !input.ExpiresAt.After(nowUTC().Truncate(time.Second)) {
		return orgapikey.OrganizationAPIKey{}, "", NewOrganizationAPIKeyExpiresAtInPastError()
	}

	org, err := s.organizationRepo.FindByID(ctx, input.ProductID, input.OrganizationID)
	if err != nil {
		return orgapikey.OrganizationAPIKey{}, "", apierror.ErrUnexpected
	}
	if org == nil {
		return orgapikey.OrganizationAPIKey{}, "", apierror.ErrNotFound
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
		return orgapikey.OrganizationAPIKey{}, "", apierror.ErrUnexpected
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

	organizationAPIKey.Permissions = slicex.Map(
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

	var created orgapikey.OrganizationAPIKey
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.apiKeyRepo.Create(txCtx, organizationAPIKey)
		if createErr != nil {
			return createErr
		}

		if enqueueErr := s.enqueueExpirationEvent(txCtx, created); enqueueErr != nil {
			return enqueueErr
		}

		return nil
	})
	if err != nil {
		logger.Error().
			Str("organization_api_key_id", organizationAPIKey.ID).
			Str("organization_id", input.OrganizationID).
			Err(err).
			Msg("failed to create organization API key")
		return orgapikey.OrganizationAPIKey{}, "", apierror.ErrUnexpected
	}

	return created, clearAPIKey, nil
}

func (s *organizationAPIKeyService) GetByID(
	ctx context.Context,
	input orgapikey.GetOrganizationAPIKeyInput,
) (*orgapikey.OrganizationAPIKey, error) {
	logger := s.logger.With().Str("operation", "GetByID").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}
	if err := s.ensureOrganizationBelongsToProduct(ctx, input.ProductID, input.OrganizationID); err != nil {
		return nil, err
	}

	apiKey, err := s.apiKeyRepo.GetByID(ctx, input.OrganizationID, input.ID)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get organization API key")
		return nil, apierror.ErrUnexpected
	}

	if apiKey == nil {
		return nil, apierror.ErrNotFound
	}

	return apiKey, nil
}

func (s *organizationAPIKeyService) Update(
	ctx context.Context,
	input orgapikey.UpdateOrganizationAPIKeyInput,
) (orgapikey.OrganizationAPIKey, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := validateStruct(input); err != nil {
		return orgapikey.OrganizationAPIKey{}, err
	}
	if err := s.ensureOrganizationBelongsToProduct(ctx, input.ProductID, input.OrganizationID); err != nil {
		return orgapikey.OrganizationAPIKey{}, err
	}

	existingAPIKey, err := s.apiKeyRepo.GetByID(ctx, input.OrganizationID, input.ID)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get organization API key for update")
		return orgapikey.OrganizationAPIKey{}, apierror.ErrUnexpected
	}

	if existingAPIKey == nil {
		return orgapikey.OrganizationAPIKey{}, apierror.ErrNotFound
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

	updated, err := s.apiKeyRepo.Update(ctx, updatedAPIKey)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to update organization API key")
		return orgapikey.OrganizationAPIKey{}, apierror.ErrUnexpected
	}

	return updated, nil
}

func (s *organizationAPIKeyService) Search(
	ctx context.Context,
	input orgapikey.SearchOrganizationAPIKeysInput,
) (*search.Result[orgapikey.OrganizationAPIKey], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}
	if err := s.ensureOrganizationBelongsToProduct(ctx, input.ProductID, input.OrganizationID); err != nil {
		return nil, err
	}

	result, err := s.apiKeyRepo.SearchByOrganizationID(ctx, input)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Err(err).
			Msg("failed to search organization API keys")
		return nil, apierror.ErrUnexpected
	}

	return &result, nil
}

func (s *organizationAPIKeyService) Delete(
	ctx context.Context,
	input orgapikey.DeleteOrganizationAPIKeyInput,
) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := validateStruct(input); err != nil {
		return err
	}
	if err := s.ensureOrganizationBelongsToProduct(ctx, input.ProductID, input.OrganizationID); err != nil {
		return err
	}

	existingAPIKey, err := s.apiKeyRepo.GetByID(ctx, input.OrganizationID, input.ID)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get organization API key for deletion")
		return apierror.ErrUnexpected
	}

	if existingAPIKey == nil {
		return apierror.ErrNotFound
	}

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if deleteErr := s.apiKeyRepo.Delete(txCtx, input.OrganizationID, input.ID); deleteErr != nil {
			logger.Error().
				Str("organization_id", input.OrganizationID).
				Str("api_key_id", input.ID).
				Err(deleteErr).
				Msg("failed to delete organization API key")
			return apierror.ErrUnexpected
		}

		if existingAPIKey.ExpiresAt == nil {
			return nil
		}

		jobs, listErr := s.queue.ListJobs(txCtx, pgqueue.ListJobsParams{
			QueueName: organizationAPIKeyEventQueueName,
			Status:    pgqueue.StatusPending,
			Limit:     organizationAPIKeyEventListLimit,
		})
		if listErr != nil {
			logger.Error().
				Str("api_key_id", input.ID).
				Err(listErr).
				Msg("failed to list pending queue jobs for api key deletion")
			return apierror.ErrUnexpected
		}

		for _, job := range jobs {
			var p organizationAPIKeyEventPayload
			if unmarshalErr := json.Unmarshal(job.Payload, &p); unmarshalErr != nil {
				continue
			}
			if p.APIKeyID != input.ID {
				continue
			}
			if cancelErr := s.queue.DeleteJob(txCtx, job.ID); cancelErr != nil {
				logger.Error().
					Str("api_key_id", input.ID).
					Int64("job_id", job.ID).
					Err(cancelErr).
					Msg("failed to cancel queue job for deleted api key")
				return apierror.ErrUnexpected
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

	if err := validateStruct(input); err != nil {
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
	foundAPIKey, err := s.apiKeyRepo.GetByOrganizationIDAndHashedValue(ctx, organizationID, hashedKey)
	if err != nil {
		logger.Error().Str("organization_id", organizationID).Err(err).Msg("failed to validate organization API key")
		return orgapikey.OrganizationAPIKey{}, false, apierror.ErrUnexpected
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

	_, err = s.queue.EnqueueTx(ctx, transactor.CurrentTx(ctx), pgqueue.EnqueueParams{
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
	existingAPIKey, err := s.apiKeyRepo.GetByOrganizationIDAndName(ctx, organizationID, name)
	if err != nil {
		logger.Error().
			Str("organization_id", organizationID).
			Str("name", name).
			Err(err).
			Msg("failed to search for organization API keys by name")
		return apierror.ErrUnexpected
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
	org, err := s.organizationRepo.FindByID(ctx, productID, organizationID)
	if err != nil {
		return apierror.ErrUnexpected
	}
	if org == nil {
		return apierror.ErrNotFound
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
		ctx, productID, permissionNames,
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
		missingNames := slicex.StringDiff(permissionNames, foundNames)
		return NewPermissionsNotFoundError(productID, missingNames)
	}
	return nil
}

func resourcePermissionsToStrings(
	input []resourcepermission.ProductResourcePermission,
) []string {
	return slicex.Map(
		input,
		func(permission resourcepermission.ProductResourcePermission) string {
			return permission.Name
		},
	)
}
