package service

import (
	"context"
	"time"

	apierror "github.com/nanostack-dev/nanostack-framework/pkg/apierror"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/permission"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
)

type PermissionService interface {
	Create(
		ctx context.Context, input permission.CreateProductPermissionInput,
	) (permission.ProductPermission, error)
	Update(
		ctx context.Context, input permission.UpdateProductPermissionInput,
	) (permission.ProductPermission, error)
	Delete(
		ctx context.Context, input permission.DeleteProductPermissionInput,
	) error
	FindByProductAndPermissionName(
		ctx context.Context, input permission.FindProductPermissionInput,
	) (*permission.ProductPermission, error)
	SearchByProductID(
		ctx context.Context, input permission.SearchProductPermissionInput,
	) (search.Result[permission.ProductPermission], error)
	CheckPermissionExists(
		ctx context.Context, productID string, permissions []string,
	) (bool, error)
}

type permissionService struct {
	permissionRepo repository.ProductPermissionRepository
	logger         zerolog.Logger
}

func NewPermissionService(
	permissionRepo repository.ProductPermissionRepository,
	logger zerolog.Logger,
) PermissionService {
	return &permissionService{
		permissionRepo: permissionRepo,
		logger: logger.With().Str(
			"component", "permission_service",
		).Logger(),
	}
}

func (s *permissionService) Create(
	ctx context.Context, input permission.CreateProductPermissionInput,
) (permission.ProductPermission, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := validateStruct(input); err != nil {
		return permission.ProductPermission{}, err
	}

	exists, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to check permission uniqueness")
		return permission.ProductPermission{}, apierror.ErrUnexpected
	}

	if exists != nil {
		return permission.ProductPermission{}, permission.NewPermissionNameDuplicateError(
			input.Name, input.ProductID,
		)
	}

	now := time.Now()
	perm := permission.ProductPermission{
		ProductID:   input.ProductID,
		Name:        input.Name,
		Description: input.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	createdPerm, err := s.permissionRepo.Create(ctx, perm, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to create permission")
		return permission.ProductPermission{}, apierror.ErrUnexpected
	}

	logger.Info().
		Str("product_id", input.ProductID).
		Str("name", createdPerm.Name).
		Msg("permission created")

	return createdPerm, nil
}

func (s *permissionService) Update(
	ctx context.Context, input permission.UpdateProductPermissionInput,
) (permission.ProductPermission, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := validateStruct(input); err != nil {
		return permission.ProductPermission{}, err
	}

	existing, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to find permission for update")
		return permission.ProductPermission{}, apierror.ErrUnexpected
	}

	if existing == nil {
		return permission.ProductPermission{}, permission.ErrPermissionNotFound
	}

	updated := *existing
	updated.Description = input.Description
	updated.UpdatedAt = time.Now()

	result, err := s.permissionRepo.Update(ctx, updated, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to update permission")
		return permission.ProductPermission{}, apierror.ErrUnexpected
	}

	logger.Info().
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Msg("permission updated")

	return result, nil
}

func (s *permissionService) Delete(
	ctx context.Context, input permission.DeleteProductPermissionInput,
) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := validateStruct(input); err != nil {
		return err
	}

	exists, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to check permission existence")
		return apierror.ErrUnexpected
	}

	if exists == nil {
		return nil
	}

	roleCount, err := s.permissionRepo.CountAPIKeyAssignments(ctx, input.ProductID, input.Name, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to count role assignments for permission")
		return apierror.ErrUnexpected
	}

	if roleCount > 0 {
		return permission.NewPermissionAssignedToAPIKeysError(
			input.ProductID, input.Name, roleCount,
		)
	}

	err = s.permissionRepo.DeleteByID(ctx, input.ProductID, input.Name, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to delete permission")
		return apierror.ErrUnexpected
	}

	logger.Info().
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Msg("permission deleted")

	return nil
}

func (s *permissionService) FindByProductAndPermissionName(
	ctx context.Context, input permission.FindProductPermissionInput,
) (*permission.ProductPermission, error) {
	logger := s.logger.With().Str("operation", "FindByProductAndPermissionName").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	perm, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Err(err).
			Msg("failed to find permission")
		return nil, apierror.ErrUnexpected
	}

	if perm == nil {
		return nil, permission.ErrPermissionNotFound
	}

	return perm, nil
}

func (s *permissionService) SearchByProductID(
	ctx context.Context, input permission.SearchProductPermissionInput,
) (search.Result[permission.ProductPermission], error) {
	logger := s.logger.With().Str("operation", "SearchByProductID").Logger()

	if err := validateStruct(input); err != nil {
		return search.Result[permission.ProductPermission]{}, err
	}

	result, err := s.permissionRepo.SearchByProduct(ctx, input.ProductID, input.Request, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search permissions")
		return search.Result[permission.ProductPermission]{}, apierror.ErrUnexpected
	}

	return result, nil
}

func (s *permissionService) CheckPermissionExists(
	ctx context.Context, productID string, permissions []string,
) (bool, error) {
	logger := s.logger.With().Str("operation", "CheckPermissionExists").Logger()

	if len(permissions) == 0 {
		return false, nil
	}
	perms, err := s.permissionRepo.FindByProductIDAndPermissionNames(
		ctx, productID, permissions, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", productID).
			Int("permission_count", len(permissions)).
			Err(err).
			Msg("failed to check permissions existence")
		return false, apierror.ErrUnexpected
	}
	return len(perms) == len(permissions), nil
}
