package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"anchor/internal/applog"

	"anchor/internal/searchx"
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/modules/cache"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/domain/permission"
	"anchor/internal/domain/product/apikey"
	"anchor/internal/repository"
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
	logger         *slog.Logger
}

func NewProductAPIKeyService(
	transactor transactor.Transactor,
	apiKeyRepo repository.ProductAPIKeyRepository,
	permissionRepo repository.ProductPermissionRepository,
	apiKeys cache.Cache[apikey.ProductAPIKey], logger *slog.Logger,
) ProductAPIKeyService {
	return &productAPIKeyService{
		transactor:     transactor,
		apiKeyRepo:     apiKeyRepo,
		permissionRepo: permissionRepo,
		apiKeys:        apiKeys,
		logger:         logger.With(slog.String("component", "product_api_key_service")),
	}
}

func (s *productAPIKeyService) Create(
	ctx context.Context, input apikey.CreateProductAPIKeyInput,
) (apikey.ProductAPIKey, string, error) {
	logger := s.logger.With(slog.String("operation", "Create"))

	if err := validate.ValidateStruct(input); err != nil {
		return apikey.ProductAPIKey{}, "", err
	}

	if err := s.nameUniqueValidation(ctx, input.ProductID, input.Name, "", logger); err != nil {
		return apikey.ProductAPIKey{}, "", err
	}

	clearAPIKey, err := security.GenerateProductAPIKey()
	if err != nil {
		logger.ErrorContext(ctx, "failed to generate API key", log.Err(err))
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

	logger.InfoContext(ctx, "creating product API key",
		slog.String("product_api_key_id", productAPIKey.ID),
		slog.String("product_id", input.ProductID),
		slog.String("name", input.Name),
	)

	var apiKeyPermissions []apikey.ProductAPIKeyPermission
	if len(input.Permissions) > 0 {
		canonicalPermissions, permErr := s.permissionsValidation(
			ctx, input.ProductID, input.Permissions, logger,
		)
		if permErr != nil {
			return apikey.ProductAPIKey{}, "", permErr
		}

		apiKeyPermissions = functional.Slice(
			canonicalPermissions).Map(
			func(perm string) apikey.ProductAPIKeyPermission {
				return apikey.ProductAPIKeyPermission{
					APIKeyID:       productAPIKey.ID,
					ProductID:      input.ProductID,
					PermissionName: perm,
					CreatedAt:      time.Now(),
				}
			})
	}

	productAPIKey.Permissions = apiKeyPermissions

	created, err := s.apiKeyRepo.Create(ctx, productAPIKey)
	if errors.Is(err, repository.ErrProductAPIKeyNameExists) {
		return apikey.ProductAPIKey{}, "", NewProductAPIKeyNameExistsError(input.Name, input.ProductID)
	}
	if err != nil {
		logger.ErrorContext(ctx, "failed to create product API key",
			slog.String("product_api_key_id", productAPIKey.ID),
			slog.String("product_id", input.ProductID),
			log.Err(err),
		)
		return apikey.ProductAPIKey{}, "", fault.ErrUnexpected
	}

	logger.InfoContext(ctx, "product API key created",
		slog.String("product_api_key_id", created.ID),
		slog.String("product_id", input.ProductID),
		slog.String("name", input.Name),
	)

	return created, clearAPIKey, nil
}

func (s *productAPIKeyService) GetByID(
	ctx context.Context, input apikey.GetProductAPIKeyInput,
) (*apikey.ProductAPIKey, error) {
	logger := s.logger.With(slog.String("operation", "GetByID"))

	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}

	found, err := s.apiKeyRepo.GetByID(ctx, input.ProductID, input.ID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get API key",
			slog.String("product_id", input.ProductID),
			slog.String("api_key_id", input.ID),
			log.Err(err),
		)
		return nil, fault.ErrUnexpected
	}

	if found.IsAbsent() {
		return nil, apikey.ErrProductAPIKeyNotFound
	}

	return found.ToPtr(), nil
}

func (s *productAPIKeyService) Update(
	ctx context.Context, input apikey.UpdateProductAPIKeyInput,
) (apikey.ProductAPIKey, error) {
	logger := s.logger.With(slog.String("operation", "Update"))

	if err := validate.ValidateStruct(input); err != nil {
		return apikey.ProductAPIKey{}, err
	}
	logger.InfoContext(ctx, "updating product API key",
		slog.String("product_api_key_id", input.ID),
		slog.String("product_id", input.ProductID),
	)

	foundAPIKey, err := s.apiKeyRepo.GetByID(ctx, input.ProductID, input.ID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get API key for update",
			slog.String("product_id", input.ProductID),
			slog.String("api_key_id", input.ID),
			log.Err(err),
		)
		return apikey.ProductAPIKey{}, fault.ErrUnexpected
	}

	if foundAPIKey.IsAbsent() {
		return apikey.ProductAPIKey{}, apikey.ErrProductAPIKeyNotFound
	}
	existingAPIKey := foundAPIKey.Value()

	updatedAPIKey := existingAPIKey
	permissionsUpdated := input.Permissions != nil

	if input.Name != nil && *input.Name != updatedAPIKey.Name {
		if nameErr := s.nameUniqueValidation(
			ctx,
			input.ProductID,
			*input.Name,
			existingAPIKey.ID,
			logger,
		); nameErr != nil {
			return apikey.ProductAPIKey{}, nameErr
		}
		updatedAPIKey.Name = *input.Name
	}
	if input.Description != nil {
		updatedAPIKey.Description = input.Description
	}

	if permissionsUpdated {
		updatedAPIKey.Permissions, err = s.preparePermissionUpdate(ctx, input, updatedAPIKey, logger)
		if err != nil {
			return apikey.ProductAPIKey{}, err
		}
	}

	var updatedAPIKeyFromDB apikey.ProductAPIKey
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
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

		refetched, fetchErr := s.apiKeyRepo.GetByID(
			txCtx,
			input.ProductID,
			input.ID,
		)
		if fetchErr != nil {
			return fetchErr
		}
		if refetched.IsPresent() {
			updatedAPIKeyFromDB = refetched.Value()
			return nil
		}

		updatedAPIKeyFromDB = updated
		return nil
	})
	if errors.Is(err, repository.ErrProductAPIKeyNameExists) {
		return apikey.ProductAPIKey{}, NewProductAPIKeyNameExistsError(updatedAPIKey.Name, input.ProductID)
	}
	if err != nil {
		logger.ErrorContext(ctx, "failed to update API key transaction",
			slog.String("product_id", input.ProductID),
			slog.String("api_key_id", input.ID),
			log.Err(err),
		)
		return apikey.ProductAPIKey{}, fault.ErrUnexpected
	}

	err = s.apiKeys.Key(input.ProductID, existingAPIKey.HashedValue).Evict(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "failed to evict API key from cache",
			slog.String("product_id", input.ProductID),
			slog.String("api_key_id", input.ID),
			log.Err(err),
		)
	}

	logger.InfoContext(ctx, "product API key updated",
		slog.String("product_api_key_id", input.ID),
		slog.String("product_id", input.ProductID),
	)

	return updatedAPIKeyFromDB, nil
}

func (s *productAPIKeyService) preparePermissionUpdate(
	ctx context.Context, input apikey.UpdateProductAPIKeyInput, current apikey.ProductAPIKey, logger *slog.Logger,
) ([]apikey.ProductAPIKeyPermission, error) {
	if !current.Mutable {
		return nil, NewProductAPIKeyPermissionsImmutableError(input.ID)
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
			return nil, permErr
		}
		permissions = canonicalPermissions
	}

	return functional.Slice(permissions).Map(func(perm string) apikey.ProductAPIKeyPermission {
		return apikey.ProductAPIKeyPermission{
			APIKeyID:       input.ID,
			ProductID:      input.ProductID,
			PermissionName: perm,
			CreatedAt:      time.Now(),
		}
	}), nil
}

func (s *productAPIKeyService) Delete(
	ctx context.Context, input apikey.DeleteProductAPIKeyInput,
) error {
	logger := s.logger.With(slog.String("operation", "Delete"))

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	foundAPIKey, err := s.apiKeyRepo.GetByID(ctx, input.ProductID, input.ID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get API key for deletion",
			slog.String("product_id", input.ProductID),
			slog.String("api_key_id", input.ID),
			log.Err(err),
		)
		return fault.ErrUnexpected
	}

	if foundAPIKey.IsAbsent() {
		return apikey.ErrProductAPIKeyNotFound
	}
	existingAPIKey := foundAPIKey.Value()
	if deleteErr := s.apiKeyRepo.Delete(ctx, input.ProductID, input.ID); deleteErr != nil {
		logger.ErrorContext(ctx, "failed to delete API key",
			slog.String("product_id", input.ProductID),
			slog.String("api_key_id", input.ID),
			log.Err(deleteErr),
		)
		return fault.ErrUnexpected
	}
	err = s.apiKeys.Key(input.ProductID, existingAPIKey.HashedValue).Evict(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "failed to evict API key from cache",
			slog.String("product_id", input.ProductID),
			slog.String("api_key_id", input.ID),
			log.Err(err),
		)
	}

	logger.InfoContext(ctx, "product API key deleted",
		slog.String("product_api_key_id", input.ID),
		slog.String("product_id", input.ProductID),
	)

	return nil
}

func (s *productAPIKeyService) Search(
	ctx context.Context, input apikey.SearchProductAPIKeysInput,
) (*search.Result[apikey.ProductAPIKey], error) {
	logger := s.logger.With(slog.String("operation", "Search"))

	if err := input.Validate(); err != nil {
		return nil, err
	}

	result, err := s.apiKeyRepo.SearchByProductID(ctx, input)
	if err != nil {
		if searchx.IsClientError(err) {
			return nil, err
		}
		logger.ErrorContext(ctx, "failed to search API keys",
			slog.String("product_id", input.ProductID),
			log.Err(err),
		)
		return nil, fault.ErrUnexpected
	}

	return &result, nil
}

func (s *productAPIKeyService) validateAPIKey(
	ctx context.Context, productID string, apiKey string, logger *slog.Logger,
) (apikey.ProductAPIKey, error) {
	if apiKey == "" {
		return apikey.ProductAPIKey{}, ErrInvalidAPIKey
	}

	logger.DebugContext(ctx, "validating API key", slog.String("product_id", productID))

	hashedKey := security.HashSecret(apiKey)
	foundAPIKey, err := s.apiKeys.Key(productID, hashedKey).GetOrElse(
		ctx, func() (*apikey.ProductAPIKey, error) {
			found, err := s.apiKeyRepo.GetByProductIDAndHashedValue(ctx, productID, hashedKey)
			if err != nil {
				return nil, err
			}
			return found.ToPtr(), nil
		},
	)

	if err != nil {
		applog.Error(ctx, logger, err, "failed to validate API key", slog.String("product_id", productID))
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
			logger.ErrorContext(ctx, "failed to update last used timestamp",
				slog.String("product_id", productID),
				slog.String("api_key_id", foundAPIKey.ID),
				log.Err(updateErr),
			)
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
	logger := s.logger.With(slog.String("operation", "ValidateAPIKeyAndScopes"))

	validAPIKey, err := s.validateAPIKey(ctx, input.ProductID, input.APIKeyValue, logger)
	if err != nil {
		return apikey.ProductAPIKey{}, err
	}
	if len(input.Scopes) == 0 {
		logger.ErrorContext(
			ctx,
			"every route authenticated with API key must have at least one scope, please check your configuration",
			slog.String("product_id", input.ProductID),
		)
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
		logger.DebugContext(ctx, "API key does not have required scopes",
			slog.String("product_id", input.ProductID),
			slog.Any("required_scopes", requiredScopes),
		)
		return apikey.ProductAPIKey{}, NewProductAPIKeyInsufficientPermissionsError(
			validAPIKey.ID, requiredScopes, validAPIKey.ToStringsPermissions(),
		)
	}
	logger.DebugContext(ctx, "API key validated successfully with required scopes",
		slog.String("product_id", input.ProductID),
	)
	return validAPIKey, nil
}

func (s *productAPIKeyService) nameUniqueValidation(
	ctx context.Context, productID, name, ownKeyID string, logger *slog.Logger,
) error {
	found, err := s.apiKeyRepo.GetByProductIDAndName(ctx, productID, name)
	if err != nil {
		logger.ErrorContext(ctx, "failed to search for API keys by name",
			slog.String("product_id", productID),
			slog.String("name", name),
			log.Err(err),
		)
		return fault.ErrUnexpected
	}
	if found.Exists(func(holder apikey.ProductAPIKey) bool { return holder.ID != ownKeyID }) {
		return NewProductAPIKeyNameExistsError(name, productID)
	}
	return nil
}

func (s *productAPIKeyService) permissionsValidation(
	ctx context.Context, productID string, permissionNames []string, logger *slog.Logger,
) ([]string, error) {
	permsFound, err := s.permissionRepo.FindByProductIDAndPermissionNames(
		ctx, productID, permissionNames,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to find permissions by names",
			slog.String("product_id", productID),
			slog.Int("permission_count", len(permissionNames)),
			log.Err(err),
		)
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
	return functional.Slice(input).Map(func(permission permission.ProductPermission) string {
		return permission.Name
	})
}
