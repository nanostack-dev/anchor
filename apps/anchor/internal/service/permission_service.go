package service

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

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
	) (permission.ProductPermission, error)
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

	if err := validate.ValidateStruct(input); err != nil {
		return permission.ProductPermission{}, err
	}

	exists, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name,
	)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Msg("failed to check permission uniqueness")
		return permission.ProductPermission{}, fault.ErrUnexpected
	}

	if exists.IsPresent() {
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

	createdPerm, err := s.permissionRepo.Create(ctx, perm)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Msg("failed to create permission")
		return permission.ProductPermission{}, fault.ErrUnexpected
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

	if err := validate.ValidateStruct(input); err != nil {
		return permission.ProductPermission{}, err
	}

	existing, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name,
	)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Msg("failed to find permission for update")
		return permission.ProductPermission{}, fault.ErrUnexpected
	}

	if existing.IsAbsent() {
		return permission.ProductPermission{}, permission.ErrPermissionNotFound
	}

	updated := existing.Value()
	updated.Description = input.Description
	updated.UpdatedAt = time.Now()

	result, err := s.permissionRepo.Update(ctx, updated)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Msg("failed to update permission")
		return permission.ProductPermission{}, fault.ErrUnexpected
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

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	found, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name,
	)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Msg("failed to check permission existence")
		return fault.ErrUnexpected
	}

	if found.IsAbsent() {
		return nil
	}
	exists := found.Value()

	roleCount, err := s.permissionRepo.CountAPIKeyAssignments(ctx, input.ProductID, exists.Name)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Msg("failed to count role assignments for permission")
		return fault.ErrUnexpected
	}

	if roleCount > 0 {
		return permission.NewPermissionAssignedToAPIKeysError(
			input.ProductID, input.Name, roleCount,
		)
	}

	err = s.permissionRepo.DeleteByID(ctx, input.ProductID, exists.Name)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Msg("failed to delete permission")
		return fault.ErrUnexpected
	}

	logger.Info().
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Msg("permission deleted")

	return nil
}

func (s *permissionService) FindByProductAndPermissionName(
	ctx context.Context, input permission.FindProductPermissionInput,
) (permission.ProductPermission, error) {
	logger := s.logger.With().Str("operation", "FindByProductAndPermissionName").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return permission.ProductPermission{}, err
	}

	found, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name,
	)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Str("name", input.Name).
			Msg("failed to find permission")
		return permission.ProductPermission{}, fault.ErrUnexpected
	}

	return found.ToResult(permission.ErrPermissionNotFound).Value()
}

func (s *permissionService) SearchByProductID(
	ctx context.Context, input permission.SearchProductPermissionInput,
) (search.Result[permission.ProductPermission], error) {
	logger := s.logger.With().Str("operation", "SearchByProductID").Logger()

	if err := validate.ValidateStruct(input); err != nil {
		return search.Result[permission.ProductPermission]{}, err
	}

	result, err := s.permissionRepo.SearchByProduct(ctx, input.ProductID, input.Request)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", input.ProductID).
			Msg("failed to search permissions")
		return search.Result[permission.ProductPermission]{}, fault.ErrUnexpected
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
		ctx, productID, permissions,
	)
	if err != nil {
		log.Event(&logger, err).
			Str("product_id", productID).
			Int("permission_count", len(permissions)).
			Msg("failed to check permissions existence")
		return false, fault.ErrUnexpected
	}
	return len(perms) == len(permissions), nil
}
