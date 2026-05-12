package service

import (
	"context"
	"database/sql"
	"time"

	"anchor/internal/security"

	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

	"anchor/internal/domain/permission"
	"anchor/internal/domain/product/apikey"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
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
	db             *sql.DB
	apiKeyRepo     repository.ProductAPIKeyRepository
	permissionRepo repository.ProductPermissionRepository
	cacheService   ProductAPIKeyCacheService
	logger         zerolog.Logger
}

func NewProductAPIKeyService(
	db *sql.DB,
	apiKeyRepo repository.ProductAPIKeyRepository,
	permissionRepo repository.ProductPermissionRepository,
	cacheService ProductAPIKeyCacheService, logger zerolog.Logger,
) ProductAPIKeyService {
	return &productAPIKeyService{
		db:             db,
		apiKeyRepo:     apiKeyRepo,
		permissionRepo: permissionRepo,
		cacheService:   cacheService,
		logger: logger.With().Str(
			"component", "product_api_key_service",
		).Logger(),
	}
}

func (s *productAPIKeyService) Create(
	ctx context.Context, input apikey.CreateProductAPIKeyInput,
) (apikey.ProductAPIKey, string, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return apikey.ProductAPIKey{}, "", err
	}

	if err := s.nameUniqueValidation(ctx, input.ProductID, input.Name, logger); err != nil {
		return apikey.ProductAPIKey{}, "", err
	}

	clearAPIKey, err := security.GenerateProductAPIKey()
	if err != nil {
		logger.Error().Err(err).Msg("failed to generate API key")
		return apikey.ProductAPIKey{}, "", toolkit.ErrUnexpected
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
		if permErr := s.permissionsValidation(
			ctx, input.ProductID, input.Permissions, logger,
		); permErr != nil {
			return apikey.ProductAPIKey{}, "", permErr
		}

		apiKeyPermissions = toolkit.TransformSlice(
			input.Permissions, func(perm string) apikey.ProductAPIKeyPermission {
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

	created, err := s.apiKeyRepo.Create(ctx, productAPIKey, nil)
	if err != nil {
		logger.Error().
			Str("product_api_key_id", productAPIKey.ID).
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to create product API key")
		return apikey.ProductAPIKey{}, "", toolkit.ErrUnexpected
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

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}

	apiKey, err := s.apiKeyRepo.GetByID(ctx, input.ProductID, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get API key")
		return nil, toolkit.ErrUnexpected
	}

	if apiKey == nil {
		return nil, toolkit.ErrNotFound
	}

	return apiKey, nil
}

func (s *productAPIKeyService) Update(
	ctx context.Context, input apikey.UpdateProductAPIKeyInput,
) (apikey.ProductAPIKey, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return apikey.ProductAPIKey{}, err
	}
	logger.Info().
		Str("product_api_key_id", input.ID).
		Str("product_id", input.ProductID).
		Msg("updating product API key")

	existingAPIKey, err := s.apiKeyRepo.GetByID(ctx, input.ProductID, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get API key for update")
		return apikey.ProductAPIKey{}, toolkit.ErrUnexpected
	}

	if existingAPIKey == nil {
		return apikey.ProductAPIKey{}, toolkit.ErrNotFound
	}

	updatedAPIKey := *existingAPIKey
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
			if permErr := s.permissionsValidation(
				ctx,
				input.ProductID,
				permissions,
				logger,
			); permErr != nil {
				return apikey.ProductAPIKey{}, permErr
			}
		}

		updatedAPIKey.Permissions = toolkit.TransformSlice(
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

	updatedAPIKeyFromDB, err := toolkit.WithTxReturn(
		s.db,
		func(tx *sql.Tx) (apikey.ProductAPIKey, error) {
			dbOptions := &toolkit.DBOptions{Tx: tx}

			updated, updateErr := s.apiKeyRepo.Update(ctx, updatedAPIKey, dbOptions)
			if updateErr != nil {
				return apikey.ProductAPIKey{}, updateErr
			}

			if !permissionsUpdated {
				return updated, nil
			}

			if replaceErr := s.apiKeyRepo.ReplacePermissions(
				ctx,
				input.ProductID,
				input.ID,
				updatedAPIKey.Permissions,
				dbOptions,
			); replaceErr != nil {
				return apikey.ProductAPIKey{}, replaceErr
			}

			refetched, fetchErr := s.apiKeyRepo.GetByID(
				ctx,
				input.ProductID,
				input.ID,
				dbOptions,
			)
			if fetchErr != nil {
				return apikey.ProductAPIKey{}, fetchErr
			}
			if refetched != nil {
				return *refetched, nil
			}

			return updated, nil
		},
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to update API key transaction")
		return apikey.ProductAPIKey{}, toolkit.ErrUnexpected
	}

	err = s.cacheService.EvictAPIKeyByHashedValue(ctx, input.ProductID, existingAPIKey.HashedValue)
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

	if err := toolkit.ValidateStruct(input); err != nil {
		return err
	}

	existingAPIKey, err := s.apiKeyRepo.GetByID(ctx, input.ProductID, input.ID, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(err).
			Msg("failed to get API key for deletion")
		return toolkit.ErrUnexpected
	}

	if existingAPIKey == nil {
		return toolkit.ErrNotFound
	}
	if deleteErr := s.apiKeyRepo.Delete(ctx, input.ProductID, input.ID, nil); deleteErr != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("api_key_id", input.ID).
			Err(deleteErr).
			Msg("failed to delete API key")
		return toolkit.ErrUnexpected
	}
	err = s.cacheService.EvictAPIKeyByHashedValue(
		ctx, input.ProductID, existingAPIKey.HashedValue,
	)
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

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}

	result, err := s.apiKeyRepo.SearchByProductID(ctx, input, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search API keys")
		return nil, toolkit.ErrUnexpected
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
	foundAPIKey, err := s.cacheService.GetOrElseAPIKeyHashedValue(
		ctx, productID, hashedKey, func() (*apikey.ProductAPIKey, error) {
			return s.apiKeyRepo.GetByProductIDAndHashedValue(ctx, productID, hashedKey, nil)
		},
	)

	if err != nil {
		logger.Error().Str("product_id", productID).Err(err).Msg("failed to validate API key")
		return apikey.ProductAPIKey{}, toolkit.ErrUnexpected
	}

	if foundAPIKey == nil {
		return apikey.ProductAPIKey{}, ErrInvalidAPIKey
	}

	shouldUpdate := foundAPIKey.LastUsedAt == nil || time.Since(*foundAPIKey.LastUsedAt) > time.Hour
	if shouldUpdate {
		if updateErr := s.apiKeyRepo.UpdateLastUsedAt(
			ctx, foundAPIKey.ProductID, foundAPIKey.ID,
			nil,
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
		return apikey.ProductAPIKey{}, toolkit.ErrUnexpected
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
	existingAPIKey, err := s.apiKeyRepo.GetByProductIDAndName(ctx, productID, name, nil)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Str("name", name).
			Err(err).
			Msg("failed to search for API keys by name")
		return toolkit.ErrUnexpected
	}
	if existingAPIKey != nil {
		return NewProductAPIKeyNameExistsError(name, productID)
	}
	return nil
}

func (s *productAPIKeyService) permissionsValidation(
	ctx context.Context, productID string, permissionNames []string, logger zerolog.Logger,
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
		foundNames := s.permissionsToStrings(permsFound)
		missingNames := toolkit.StringSliceDiff(foundNames, permissionNames)
		return NewPermissionsNotFoundError(productID, missingNames)
	}
	return nil
}

func (s *productAPIKeyService) permissionsToStrings(
	input []permission.ProductPermission,
) []string {
	return toolkit.TransformSlice(
		input, func(permission permission.ProductPermission) string {
			return permission.Name
		},
	)
}
