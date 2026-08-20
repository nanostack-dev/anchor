package service

import (
	"context"
	"time"

	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/modules/cache"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	"anchor/internal/domain/permission"
	"anchor/internal/domain/product/apikey"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
)

const (
	apiKeyCacheTTL    = 15 * time.Minute
	apiKeyCachePrefix = "product_apikey"
)

type ProductAPIKeyService interface {
	Create(
		ctx context.Context, input apikey.CreateProductAPIKeyInput,
	) (apikey.ProductAPIKey, string, error)
	GetByID(
		ctx context.Context, input apikey.GetProductAPIKeyInput,
	) (*apikey.ProductAPIKey, error)
	Update(
		ctx context.Context, input apikey.UpdateProductAPIKeyInput,
	) (apikey.ProductAPIKey, error)
	Delete(
		ctx context.Context, input apikey.DeleteProductAPIKeyInput,
	) error
	Search(
		ctx context.Context, input apikey.SearchProductAPIKeysInput,
	) (*search.Result[apikey.ProductAPIKey], error)
	ValidateAPIKeyAndScopes(
		ctx context.Context, input apikey.ValidateAPIKeyScopesInput,
	) (apikey.ProductAPIKey, error)
}

type productAPIKeyService struct {
	transactor     transactor.Transactor
	apiKeyRepo     repository.ProductAPIKeyRepository
	permissionRepo repository.ProductPermissionRepository
	apiKeys        cache.Cache[apikey.ProductAPIKey]
	logger         zerolog.Logger
}

func NewProductAPIKeyService(
	transactor transactor.Transactor,
	apiKeyRepo repository.ProductAPIKeyRepository,
	permissionRepo repository.ProductPermissionRepository,
	cacheStore cache.Store, logger zerolog.Logger,
) ProductAPIKeyService {
	return &productAPIKeyService{
		transactor:     transactor,
		apiKeyRepo:     apiKeyRepo,
		permissionRepo: permissionRepo,
		apiKeys:        cache.New[apikey.ProductAPIKey](cacheStore, apiKeyCachePrefix, apiKeyCacheTTL, logger),
		logger: logger.With().Str(
			"component", "product_api_key_service",
		).Logger(),
	}
}

func (s *productAPIKeyService) Create(
	ctx context.Context, input apikey.CreateProductAPIKeyInput,
) (apikey.ProductAPIKey, string, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := validateStruct(input); err != nil {
		return apikey.ProductAPIKey{}, "", err
	}

	if err := s.nameUniqueValidation(ctx, input.ProductID, input.Name, logger); err != nil {
		return apikey.ProductAPIKey{}, "", err
	}

	clearAPIKey, err := security.GenerateProductAPIKey()
	if err != nil {
		logger.Error().Err(err).Msg("failed to generate API key")
		return apikey.ProductAPIKey{}, "", fault.ErrUnexpected
	}

	hashedValue := security.HashSecret(clearAPIKey)
	obfuscatedValue := security.ObfuscateProductAPIKey(clearAPIKey)

	productAPIKey := apikey.ProductAPIKey{
		ProductID:       input.ProductID,
		Name:            input.Name,
		Description:     input.Description,
		Mutable:         input.Mutable,
		HashedValue:     hashedValue,
		ObfuscatedValue: obfuscatedValue,
		Status:          apikey.StatusActive,
	}
	productAPIKey.GenerateID()

	logger.Info().
		Str("product_api_key_id", productAPIKey.ID).
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Msg("creating product API key")

	var apiKeyPermissions []apikey.ProductAPIKeyPermission
	if len(input.Permissions) > 0 {
		canonicalPermissions, permErr := s.permissionsValidation(
			ctx, input.ProductID, input.Permissions, logger,
		)
		if permErr != nil {
			return apikey.ProductAPIKey{}, "", permErr
		}

		apiKeyPermissions = slicex.Map(
			canonicalPermissions, func(perm string) apikey.ProductAPIKeyPermission {
				return apikey.ProductAPIKeyPermission{
					APIKeyID:       productAPIKey.ID,
					ProductID:      input.ProductID,
					PermissionName: perm,
					CreatedAt:      time.Now(),
				}
			},
		)
	}

	productAPIKey.Permissions = apiKeyPermissions

	created, err := s.apiKeyRepo.Create(ctx, productAPIKey)
	if err != nil {
		logger.Error().
			Str("product_api_key_id", productAPIKey.ID).
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to create product API key")
		return apikey.ProductAPIKey{}, "", fault.ErrUnexpected
	}

	logger.Info().
		Str("product_api_key_id", created.ID).
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Msg("product API key created")

	return created, clearAPIKey, nil
}

func (s *productAPIKeyService) GetByID(
	ctx context.Context, input apikey.GetProductAPIKeyInput,
) (*apikey.ProductAPIKey, error) {
	logger := s.logger.With().Str("operation", "GetByID").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	found := s.apiKeyRepo.GetByID(ctx, input.ProductID, input.ID)
	if err := found.Err(); err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get API key")
		return nil, fault.ErrUnexpected
	}

	if !found.IsPresent() {
		return nil, fault.ErrNotFound
	}

	return found.ToPtr(), nil
}

func (s *productAPIKeyService) Update(
	ctx context.Context, input apikey.UpdateProductAPIKeyInput,
) (apikey.ProductAPIKey, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := validateStruct(input); err != nil {
		return apikey.ProductAPIKey{}, err
	}
	logger.Info().
		Str("product_api_key_id", input.ID).
		Str("product_id", input.ProductID).
		Msg("updating product API key")

	foundAPIKey := s.apiKeyRepo.GetByID(ctx, input.ProductID, input.ID)
	if err := foundAPIKey.Err(); err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get API key for update")
		return apikey.ProductAPIKey{}, fault.ErrUnexpected
	}

	if !foundAPIKey.IsPresent() {
		return apikey.ProductAPIKey{}, fault.ErrNotFound
	}
	existingAPIKey := foundAPIKey.Value()

	updatedAPIKey := existingAPIKey
	permissionsUpdated := input.Permissions != nil

	if input.Name != nil && *input.Name != updatedAPIKey.Name {
		if nameErr := s.nameUniqueValidation(ctx, input.ProductID, *input.Name, logger); nameErr != nil {
			return apikey.ProductAPIKey{}, nameErr
		}
		updatedAPIKey.Name = *input.Name
	}
	if input.Description != nil {
		updatedAPIKey.Description = input.Description
	}

	if permissionsUpdated {
		if !updatedAPIKey.Mutable {
			return apikey.ProductAPIKey{}, NewProductAPIKeyPermissionsImmutableError(input.ID)
		}

		permissions := *input.Permissions
		if len(permissions) > 0 {
			canonicalPermissions, permErr := s.permissionsValidation(
				ctx,
				input.ProductID,
				permissions,
				logger,
			)
			if permErr != nil {
				return apikey.ProductAPIKey{}, permErr
			}
			permissions = canonicalPermissions
		}

		updatedAPIKey.Permissions = slicex.Map(
			permissions,
			func(perm string) apikey.ProductAPIKeyPermission {
				return apikey.ProductAPIKeyPermission{
					APIKeyID:       input.ID,
					ProductID:      input.ProductID,
					PermissionName: perm,
					CreatedAt:      time.Now(),
				}
			},
		)
	}

	var updatedAPIKeyFromDB apikey.ProductAPIKey
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		updated, updateErr := s.apiKeyRepo.Update(txCtx, updatedAPIKey)
		if updateErr != nil {
			return updateErr
		}

		if !permissionsUpdated {
			updatedAPIKeyFromDB = updated
			return nil
		}

		if replaceErr := s.apiKeyRepo.ReplacePermissions(
			txCtx,
			input.ProductID,
			input.ID,
			updatedAPIKey.Permissions,
		); replaceErr != nil {
			return replaceErr
		}

		refetched := s.apiKeyRepo.GetByID(
			txCtx,
			input.ProductID,
			input.ID,
		)
		if fetchErr := refetched.Err(); fetchErr != nil {
			return fetchErr
		}
		if refetched.IsPresent() {
			updatedAPIKeyFromDB = refetched.Value()
			return nil
		}

		updatedAPIKeyFromDB = updated
		return nil
	})
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to update API key transaction")
		return apikey.ProductAPIKey{}, fault.ErrUnexpected
	}

	err = s.apiKeys.Key(input.ProductID, existingAPIKey.HashedValue).Evict(ctx)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to evict API key from cache")
	}

	logger.Info().
		Str("product_api_key_id", input.ID).
		Str("product_id", input.ProductID).
		Msg("product API key updated")

	return updatedAPIKeyFromDB, nil
}

func (s *productAPIKeyService) Delete(
	ctx context.Context, input apikey.DeleteProductAPIKeyInput,
) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := validateStruct(input); err != nil {
		return err
	}

	foundAPIKey := s.apiKeyRepo.GetByID(ctx, input.ProductID, input.ID)
	if err := foundAPIKey.Err(); err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get API key for deletion")
		return fault.ErrUnexpected
	}

	if !foundAPIKey.IsPresent() {
		return fault.ErrNotFound
	}
	existingAPIKey := foundAPIKey.Value()
	if deleteErr := s.apiKeyRepo.Delete(ctx, input.ProductID, input.ID); deleteErr != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(deleteErr).
			Msg("failed to delete API key")
		return fault.ErrUnexpected
	}
	err := s.apiKeys.Key(input.ProductID, existingAPIKey.HashedValue).Evict(ctx)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to evict API key from cache")
	}

	logger.Info().
		Str("product_api_key_id", input.ID).
		Str("product_id", input.ProductID).
		Msg("product API key deleted")

	return nil
}

func (s *productAPIKeyService) Search(
	ctx context.Context, input apikey.SearchProductAPIKeysInput,
) (*search.Result[apikey.ProductAPIKey], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	result, err := s.apiKeyRepo.SearchByProductID(ctx, input)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search API keys")
		return nil, fault.ErrUnexpected
	}

	return &result, nil
}

func (s *productAPIKeyService) validateAPIKey(
	ctx context.Context, productID string, apiKey string, logger zerolog.Logger,
) (apikey.ProductAPIKey, error) {
	if apiKey == "" {
		return apikey.ProductAPIKey{}, ErrInvalidAPIKey
	}

	logger.Debug().Str("product_id", productID).Msg("validating API key")

	hashedKey := security.HashSecret(apiKey)
	foundAPIKey, err := s.apiKeys.Key(productID, hashedKey).GetOrElse(
		ctx, func() (*apikey.ProductAPIKey, error) {
			found := s.apiKeyRepo.GetByProductIDAndHashedValue(ctx, productID, hashedKey)
			if err := found.Err(); err != nil {
				return nil, err
			}
			return found.ToPtr(), nil
		},
	)

	if err != nil {
		logger.Error().Str("product_id", productID).Err(err).Msg("failed to validate API key")
		return apikey.ProductAPIKey{}, fault.ErrUnexpected
	}

	if foundAPIKey == nil {
		return apikey.ProductAPIKey{}, ErrInvalidAPIKey
	}

	shouldUpdate := foundAPIKey.LastUsedAt == nil || time.Since(*foundAPIKey.LastUsedAt) > time.Hour
	if shouldUpdate {
		if updateErr := s.apiKeyRepo.UpdateLastUsedAt(
			ctx, foundAPIKey.ProductID, foundAPIKey.ID,
		); updateErr != nil {
			logger.Error().
				Str("product_id", productID).
				Str("api_key_id", foundAPIKey.ID).
				Err(updateErr).
				Msg("failed to update last used timestamp")
		}
	}
	if foundAPIKey.Status != apikey.StatusActive {
		return apikey.ProductAPIKey{}, NewProductAPIKeyInactiveError(foundAPIKey.ID)
	}

	return *foundAPIKey, nil
}

func (s *productAPIKeyService) ValidateAPIKeyAndScopes(
	ctx context.Context, input apikey.ValidateAPIKeyScopesInput,
) (apikey.ProductAPIKey, error) {
	logger := s.logger.With().Str("operation", "ValidateAPIKeyAndScopes").Logger()

	validAPIKey, err := s.validateAPIKey(ctx, input.ProductID, input.APIKeyValue, logger)
	if err != nil {
		return apikey.ProductAPIKey{}, err
	}
	if len(input.Scopes) == 0 {
		logger.Error().
			Str("product_id", input.ProductID).
			Msg("every route authenticated with API key must have at least one scope, please check your configuration")
		return apikey.ProductAPIKey{}, fault.ErrUnexpected
	}

	permissionMap := make(map[string]bool)
	for _, perm := range validAPIKey.Permissions {
		permissionMap[perm.PermissionName] = true
	}

	var requiredScopes []string
	for _, scope := range input.Scopes {
		if !permissionMap[scope] {
			requiredScopes = append(requiredScopes, scope)
		}
	}

	if len(requiredScopes) > 0 {
		logger.Debug().
			Str("product_id", input.ProductID).
			Strs("required_scopes", requiredScopes).
			Msg("API key does not have required scopes")
		return apikey.ProductAPIKey{}, NewProductAPIKeyInsufficientPermissionsError(
			validAPIKey.ID, requiredScopes, validAPIKey.ToStringsPermissions(),
		)
	}
	logger.Debug().Str("product_id", input.ProductID).Msg("API key validated successfully with required scopes")
	return validAPIKey, nil
}

func (s *productAPIKeyService) nameUniqueValidation(
	ctx context.Context, productID, name string, logger zerolog.Logger,
) error {
	found := s.apiKeyRepo.GetByProductIDAndName(ctx, productID, name)
	if err := found.Err(); err != nil {
		logger.Error().
			Str("product_id", productID).
			Str("name", name).
			Err(err).
			Msg("failed to search for API keys by name")
		return fault.ErrUnexpected
	}
	if found.IsPresent() {
		return NewProductAPIKeyNameExistsError(name, productID)
	}
	return nil
}

func (s *productAPIKeyService) permissionsValidation(
	ctx context.Context, productID string, permissionNames []string, logger zerolog.Logger,
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
	foundNames := s.permissionsToNames(permsFound)
	canonicalPermissions, missingNames := canonicalizePermissionNames(foundNames, permissionNames)
	if len(missingNames) > 0 {
		return nil, NewPermissionsNotFoundError(productID, missingNames)
	}

	return canonicalPermissions, nil
}

func (s *productAPIKeyService) permissionsToNames(input []permission.ProductPermission) []string {
	return slicex.Map(input, func(permission permission.ProductPermission) string {
		return permission.Name
	})
}
