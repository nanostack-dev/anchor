package service

import (
	"context"
	"time"

	apierror "github.com/nanostack-dev/nanostack-framework/pkg/apierror"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
)

type ResourcePermissionService interface {
	Create(
		ctx context.Context, input resourcepermission.CreateProductResourcePermissionInput,
	) (resourcepermission.ProductResourcePermission, error)

	GetByID(
		ctx context.Context, input resourcepermission.GetProductResourcePermissionInput,
	) (*resourcepermission.ProductResourcePermission, error)

	Update(
		ctx context.Context, input resourcepermission.UpdateProductResourcePermissionInput,
	) (resourcepermission.ProductResourcePermission, error)

	Delete(
		ctx context.Context, input resourcepermission.DeleteProductResourcePermissionInput,
	) error

	SearchByProduct(
		ctx context.Context, input resourcepermission.SearchProductResourcePermissionInput,
	) (search.Result[resourcepermission.ProductResourcePermission], error)
	GetByRole(
		ctx context.Context, input resourcepermission.GetProductRoleResourcePermissionsInput,
	) ([]resourcepermission.ProductResourcePermission, error)
}

type resourcePermissionService struct {
	resourcePermissionRepo repository.ProductResourcePermissionRepository
	apiKeyRepo             repository.ProductAPIKeyRepository
	transactor             transactor.Transactor
	logger                 zerolog.Logger
}

func NewResourcePermissionService(
	resourcePermissionRepo repository.ProductResourcePermissionRepository,
	apiKeyRepo repository.ProductAPIKeyRepository,
	transactor transactor.Transactor,
	logger zerolog.Logger,
) ResourcePermissionService {
	return &resourcePermissionService{
		resourcePermissionRepo: resourcePermissionRepo,
		apiKeyRepo:             apiKeyRepo,
		transactor:             transactor,
		logger: logger.With().Str(
			"component", "resource_permission_service",
		).Logger(),
	}
}

func (s *resourcePermissionService) Create(
	ctx context.Context, input resourcepermission.CreateProductResourcePermissionInput,
) (resourcepermission.ProductResourcePermission, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := validateStruct(input); err != nil {
		return resourcepermission.ProductResourcePermission{}, err
	}

	now := time.Now()
	resourcePermission := resourcepermission.ProductResourcePermission{
		ProductID:     input.ProductID,
		Name:          input.Name,
		Description:   input.Description,
		ScopeModifier: input.ScopeModifier,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	permByName, err := s.resourcePermissionRepo.FindByName(ctx, input.ProductID, input.Name)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to check existing resource permission")
		return resourcepermission.ProductResourcePermission{}, apierror.ErrUnexpected
	}
	if permByName != nil {
		return resourcepermission.ProductResourcePermission{}, NewResourcePermissionAlreadyExistsError(
			input.Name,
		)
	}

	created, err := s.resourcePermissionRepo.Create(ctx, resourcePermission)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to create resource permission")
		return resourcepermission.ProductResourcePermission{}, apierror.ErrUnexpected
	}

	logger.Info().
		Str("product_id", input.ProductID).
		Str("name", created.Name).
		Msg("resource permission created")

	return created, nil
}

func (s *resourcePermissionService) GetByID(
	ctx context.Context, input resourcepermission.GetProductResourcePermissionInput,
) (*resourcepermission.ProductResourcePermission, error) {
	logger := s.logger.With().Str("operation", "GetByID").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	resourcePermission, err := s.resourcePermissionRepo.FindByName(
		ctx, input.ProductID, input.PermissionName,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("permission_name", input.PermissionName).
			Err(err).
			Msg("failed to get resource permission")
		return nil, apierror.ErrUnexpected
	}

	return resourcePermission, nil
}

func (s *resourcePermissionService) Update(
	ctx context.Context, input resourcepermission.UpdateProductResourcePermissionInput,
) (resourcepermission.ProductResourcePermission, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := validateStruct(input); err != nil {
		return resourcepermission.ProductResourcePermission{}, err
	}

	// Get existing resource permission
	existing, err := s.resourcePermissionRepo.FindByName(ctx, input.ProductID, input.Name)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to find existing resource permission")
		return resourcepermission.ProductResourcePermission{}, apierror.ErrUnexpected
	}

	if existing == nil {
		return resourcepermission.ProductResourcePermission{}, apierror.ErrNotFound
	}

	updated := *existing
	updated.UpdatedAt = time.Now()
	updated.Description = input.Description

	result, err := s.resourcePermissionRepo.Update(ctx, updated)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to update resource permission")
		return resourcepermission.ProductResourcePermission{}, apierror.ErrUnexpected
	}

	logger.Info().
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Msg("resource permission updated")

	return result, nil
}

func (s *resourcePermissionService) Delete(
	ctx context.Context, input resourcepermission.DeleteProductResourcePermissionInput,
) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := validateStruct(input); err != nil {
		return err
	}

	name, err := s.resourcePermissionRepo.FindByName(
		ctx, input.ProductID, input.Name,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to find resource permission by name")
		return apierror.ErrUnexpected
	}
	if name == nil {
		logger.Debug().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Msg("resource permission not found for deletion")
		return apierror.ErrNotFound
	}
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if apiKeyDeleteErr := s.apiKeyRepo.DeletePermissionsByName(
			txCtx, input.ProductID, input.Name,
		); apiKeyDeleteErr != nil {
			return apiKeyDeleteErr
		}

		return s.resourcePermissionRepo.DeleteByID(
			txCtx,
			input.ProductID,
			input.Name,
		)
	})
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to delete resource permission with cascading cleanup")
		return apierror.ErrUnexpected
	}

	logger.Info().
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Msg("resource permission deleted")

	return nil
}

func (s *resourcePermissionService) SearchByProduct(
	ctx context.Context, input resourcepermission.SearchProductResourcePermissionInput,
) (search.Result[resourcepermission.ProductResourcePermission], error) {
	logger := s.logger.With().Str("operation", "SearchByProduct").Logger()

	if err := validateStruct(input); err != nil {
		return search.Result[resourcepermission.ProductResourcePermission]{}, err
	}

	result, err := s.resourcePermissionRepo.SearchByProduct(
		ctx, input.ProductID, input.Request,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search resource permissions")
		return search.Result[resourcepermission.ProductResourcePermission]{}, apierror.ErrUnexpected
	}

	return result, nil
}

func (s *resourcePermissionService) GetByRole(
	ctx context.Context, input resourcepermission.GetProductRoleResourcePermissionsInput,
) ([]resourcepermission.ProductResourcePermission, error) {
	logger := s.logger.With().Str("operation", "GetByRole").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	permissions, err := s.resourcePermissionRepo.GetByRole(ctx, input.ProductRoleID)
	if err != nil {
		logger.Error().
			Str("product_role_id", input.ProductRoleID).
			Err(err).
			Msg("failed to get resource permissions by role")
		return nil, apierror.ErrUnexpected
	}

	return permissions, nil
}
