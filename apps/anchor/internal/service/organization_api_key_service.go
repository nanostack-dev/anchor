package service

import (
	"context"
	"encoding/json"
	"time"

	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/pgkit/queue"

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
	IntrospectAPIKey(
		ctx context.Context,
		input orgapikey.IntrospectOrganizationAPIKeyInput,
	) (orgapikey.ValidateOrganizationAPIKeyScopesOutput, error)
}

type organizationAPIKeyService struct {
	transactor       transactor.Transactor
	queue            *queue.Client
	apiKeyRepo       repository.OrganizationAPIKeyRepository
	organizationRepo repository.OrganizationRepository
	productRepo      repository.ProductRepository
	permissionRepo   repository.ProductResourcePermissionRepository
	logger           zerolog.Logger
}

func NewOrganizationAPIKeyService(
	transactor transactor.Transactor,
	queueClient *queue.Client,
	apiKeyRepo repository.OrganizationAPIKeyRepository,
	organizationRepo repository.OrganizationRepository,
	productRepo repository.ProductRepository,
	permissionRepo repository.ProductResourcePermissionRepository,
	logger zerolog.Logger,
) OrganizationAPIKeyService {
	return &organizationAPIKeyService{
		transactor:       transactor,
		queue:            queueClient,
		apiKeyRepo:       apiKeyRepo,
		organizationRepo: organizationRepo,
		productRepo:      productRepo,
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

	foundOrg, err := s.organizationRepo.FindByID(ctx, input.ProductID, input.OrganizationID)
	if err != nil {
		return orgapikey.OrganizationAPIKey{}, "", fault.ErrUnexpected
	}
	if foundOrg.IsAbsent() {
		return orgapikey.OrganizationAPIKey{}, "", fault.ErrNotFound
	}
	org := foundOrg.Value()
	foundProd, err := s.productRepo.FindByIDInternal(ctx, org.ProductID)
	if err != nil {
		logger.Error().
			Str("product_id", org.ProductID).
			Err(err).
			Msg("failed to find product for organization API key config")
		return orgapikey.OrganizationAPIKey{}, "", fault.ErrUnexpected
	}
	if foundProd.IsAbsent() {
		return orgapikey.OrganizationAPIKey{}, "", fault.ErrNotFound
	}
	prod := foundProd.ToPtr()

	if nameValidationErr := s.nameUniqueValidation(
		ctx,
		input.OrganizationID,
		input.Name,
		logger,
	); nameValidationErr != nil {
		return orgapikey.OrganizationAPIKey{}, "", nameValidationErr
	}

	organizationAPIKeyPrefix := prod.Config.WithDefaults().OrganizationAPIKeys.Prefix
	clearAPIKey, err := security.GenerateOrganizationAPIKey(organizationAPIKeyPrefix)
	if err != nil {
		logger.Error().Err(err).Msg("failed to generate organization API key")
		return orgapikey.OrganizationAPIKey{}, "", fault.ErrUnexpected
	}

	hashedValue := security.HashSecret(clearAPIKey)
	obfuscatedValue := security.ObfuscateOrganizationAPIKey(organizationAPIKeyPrefix, clearAPIKey)

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

	canonicalPermissions, permissionValidationErr := s.permissionsValidation(
		ctx,
		org.ProductID,
		input.Permissions,
		logger,
	)
	if permissionValidationErr != nil {
		return orgapikey.OrganizationAPIKey{}, "", permissionValidationErr
	}

	organizationAPIKey.Permissions = functional.Slice(
		canonicalPermissions).Map(

		func(perm string) orgapikey.OrganizationAPIKeyPermission {
			return orgapikey.OrganizationAPIKeyPermission{
				APIKeyID:       organizationAPIKey.ID,
				OrganizationID: input.OrganizationID,
				ProductID:      org.ProductID,
				PermissionName: perm,
				CreatedAt:      time.Now(),
			}
		})

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
		return orgapikey.OrganizationAPIKey{}, "", fault.ErrUnexpected
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

	found, err := s.apiKeyRepo.GetByID(ctx, input.OrganizationID, input.ID)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get organization API key")
		return nil, fault.ErrUnexpected
	}

	if found.IsAbsent() {
		return nil, fault.ErrNotFound
	}

	return found.ToPtr(), nil
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

	found, err := s.apiKeyRepo.GetByID(ctx, input.OrganizationID, input.ID)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get organization API key for update")
		return orgapikey.OrganizationAPIKey{}, fault.ErrUnexpected
	}

	if found.IsAbsent() {
		return orgapikey.OrganizationAPIKey{}, fault.ErrNotFound
	}

	existingAPIKey := found.Value()
	updatedAPIKey := existingAPIKey
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
		return orgapikey.OrganizationAPIKey{}, fault.ErrUnexpected
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
		return nil, fault.ErrUnexpected
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

	found, err := s.apiKeyRepo.GetByID(ctx, input.OrganizationID, input.ID)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get organization API key for deletion")
		return fault.ErrUnexpected
	}

	if found.IsAbsent() {
		return fault.ErrNotFound
	}
	existingAPIKey := found.Value()

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if deleteErr := s.apiKeyRepo.Delete(txCtx, input.OrganizationID, input.ID); deleteErr != nil {
			logger.Error().
				Str("organization_id", input.OrganizationID).
				Str("api_key_id", input.ID).
				Err(deleteErr).
				Msg("failed to delete organization API key")
			return fault.ErrUnexpected
		}

		if existingAPIKey.ExpiresAt == nil {
			return nil
		}

		jobs, listErr := s.queue.ListJobs(txCtx, queue.ListJobsParams{
			QueueName: organizationAPIKeyEventQueueName,
			Status:    queue.StatusPending,
			Limit:     organizationAPIKeyEventListLimit,
		})
		if listErr != nil {
			logger.Error().
				Str("api_key_id", input.ID).
				Err(listErr).
				Msg("failed to list pending queue jobs for api key deletion")
			return fault.ErrUnexpected
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
				return fault.ErrUnexpected
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

	return s.buildScopesOutput(validAPIKey, inactive, input.Scopes, logger), nil
}

// IntrospectAPIKey resolves an organization API key within a product without a
// supplied organization id, returning the key's identity, organization, and
// permissions. When required scopes are provided they are checked and reported
// via MissingPrivileges.
func (s *organizationAPIKeyService) IntrospectAPIKey(
	ctx context.Context,
	input orgapikey.IntrospectOrganizationAPIKeyInput,
) (orgapikey.ValidateOrganizationAPIKeyScopesOutput, error) {
	logger := s.logger.With().Str("operation", "IntrospectAPIKey").Logger()

	if err := validateStruct(input); err != nil {
		return orgapikey.ValidateOrganizationAPIKeyScopesOutput{}, err
	}

	validAPIKey, inactive, err := s.resolveAPIKeyByProduct(ctx, input.ProductID, input.APIKeyValue, logger)
	if err != nil {
		return orgapikey.ValidateOrganizationAPIKeyScopesOutput{}, err
	}

	return s.buildScopesOutput(validAPIKey, inactive, input.Scopes, logger), nil
}

// buildScopesOutput computes the key's permission set, any missing required
// scopes, and the resulting authorization decision.
func (s *organizationAPIKeyService) buildScopesOutput(
	validAPIKey orgapikey.OrganizationAPIKey,
	inactive bool,
	requiredScopes []string,
	logger zerolog.Logger,
) orgapikey.ValidateOrganizationAPIKeyScopesOutput {
	permissionMap := make(map[string]bool, len(validAPIKey.Permissions))
	currentScopes := make([]string, 0, len(validAPIKey.Permissions))
	for _, perm := range validAPIKey.Permissions {
		permissionMap[perm.PermissionName] = true
		currentScopes = append(currentScopes, perm.PermissionName)
	}

	var missingScopes []string
	for _, scope := range requiredScopes {
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
			Str("organization_id", validAPIKey.OrganizationID).
			Str("api_key_id", validAPIKey.ID).
			Msg("organization API key is inactive")
		return output
	}

	if len(missingScopes) > 0 {
		logger.Debug().
			Str("organization_id", validAPIKey.OrganizationID).
			Strs("missing_scopes", missingScopes).
			Msg("organization API key does not have required scopes")
	}

	return output
}

// validateAPIKey resolves an organization API key within a known organization
// and evaluates its expiry/status.
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
	found, err := s.apiKeyRepo.GetByOrganizationIDAndHashedValue(ctx, organizationID, hashedKey)
	if err != nil {
		logger.Error().Str("organization_id", organizationID).Err(err).Msg("failed to validate organization API key")
		return orgapikey.OrganizationAPIKey{}, false, fault.ErrUnexpected
	}
	if found.IsAbsent() {
		return orgapikey.OrganizationAPIKey{}, false, ErrInvalidAPIKey
	}

	return s.evaluateAPIKey(ctx, found.ToPtr(), logger)
}

// resolveAPIKeyByProduct resolves an organization API key across all
// organizations within a product (the organization is derived from the key) and
// evaluates its expiry/status. Used by introspection, where the caller does not
// supply an organization id.
func (s *organizationAPIKeyService) resolveAPIKeyByProduct(
	ctx context.Context,
	productID string,
	apiKey string,
	logger zerolog.Logger,
) (orgapikey.OrganizationAPIKey, bool, error) {
	if apiKey == "" {
		return orgapikey.OrganizationAPIKey{}, false, ErrInvalidAPIKey
	}

	hashedKey := security.HashSecret(apiKey)
	found, err := s.apiKeyRepo.GetByProductIDAndHashedValueInternal(ctx, productID, hashedKey)
	if err != nil {
		logger.Error().Str("product_id", productID).Err(err).Msg("failed to introspect organization API key")
		return orgapikey.OrganizationAPIKey{}, false, fault.ErrUnexpected
	}
	if found.IsAbsent() {
		// The caller (the product) is authenticated; the subject credential simply
		// does not exist. That is a not-found, not a caller-authentication failure.
		return orgapikey.OrganizationAPIKey{}, false, fault.ErrNotFound
	}

	return s.evaluateAPIKey(ctx, found.ToPtr(), logger)
}

// evaluateAPIKey applies expiry and status handling to a resolved key,
// returning the (possibly mutated) key and whether it is inactive. It updates
// last_used_at on a live, active key and flips an expired key to inactive.
func (s *organizationAPIKeyService) evaluateAPIKey(
	ctx context.Context,
	foundAPIKey *orgapikey.OrganizationAPIKey,
	logger zerolog.Logger,
) (orgapikey.OrganizationAPIKey, bool, error) {
	organizationID := foundAPIKey.OrganizationID

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

	_, err = s.queue.EnqueueTx(ctx, transactor.CurrentTx(ctx), queue.EnqueueParams{
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
	found, err := s.apiKeyRepo.GetByOrganizationIDAndName(ctx, organizationID, name)
	if err != nil {
		logger.Error().
			Str("organization_id", organizationID).
			Str("name", name).
			Err(err).
			Msg("failed to search for organization API keys by name")
		return fault.ErrUnexpected
	}
	if found.IsPresent() {
		return NewOrganizationAPIKeyNameExistsError(name, organizationID)
	}
	return nil
}

func (s *organizationAPIKeyService) ensureOrganizationBelongsToProduct(
	ctx context.Context,
	productID, organizationID string,
) error {
	found, err := s.organizationRepo.FindByID(ctx, productID, organizationID)
	if err != nil {
		return fault.ErrUnexpected
	}
	if found.IsAbsent() {
		return fault.ErrNotFound
	}
	return nil
}

func (s *organizationAPIKeyService) permissionsValidation(
	ctx context.Context,
	productID string,
	permissionNames []string,
	logger zerolog.Logger,
) ([]string, error) {
	permsFound, err := s.permissionRepo.FindByProductIDAndPermissionNames(
		ctx, productID, permissionNames,
	)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Int("permission_count", len(permissionNames)).
			Err(err).
			Msg("failed to find permissions by names")
		return nil, err
	}
	foundNames := resourcePermissionsToNames(permsFound)
	canonicalPermissions, missingNames := canonicalizePermissionNames(foundNames, permissionNames)
	if len(missingNames) > 0 {
		return nil, NewPermissionsNotFoundError(productID, missingNames)
	}

	return canonicalPermissions, nil
}

func resourcePermissionsToNames(input []resourcepermission.ProductResourcePermission) []string {
	return functional.Slice(input).Map(func(permission resourcepermission.ProductResourcePermission) string {
		return permission.Name
	})
}
