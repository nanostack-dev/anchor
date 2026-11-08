package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/domain/permission"
	"anchor/internal/repository"
	"anchor/internal/searchx"
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
	logger         *slog.Logger
}

func NewPermissionService(
	permissionRepo repository.ProductPermissionRepository,
	logger *slog.Logger,
) PermissionService {
	return &permissionService{
		permissionRepo: permissionRepo,
		logger:         logger.With(slog.String("component", "permission_service")),
	}
}

func (s *permissionService) Create(
	ctx context.Context, input permission.CreateProductPermissionInput,
) (permission.ProductPermission, error) {
	logger := s.logger.With(slog.String("operation", "Create"))

	if err := validate.ValidateStruct(input); err != nil {
		return permission.ProductPermission{}, err
	}

	exists, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to check permission uniqueness",
			slog.String("product_id", input.ProductID),
			slog.String("name", input.Name),
			log.Err(err),
		)
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
	if errors.Is(err, repository.ErrProductPermissionExists) {
		return permission.ProductPermission{}, permission.NewPermissionNameDuplicateError(
			input.Name, input.ProductID,
		)
	}
	if err != nil {
		logger.ErrorContext(ctx, "failed to create permission",
			slog.String("product_id", input.ProductID),
			slog.String("name", input.Name),
			log.Err(err),
		)
		return permission.ProductPermission{}, fault.ErrUnexpected
	}

	logger.InfoContext(ctx, "permission created",
		slog.String("product_id", input.ProductID),
		slog.String("name", createdPerm.Name),
	)

	return createdPerm, nil
}

func (s *permissionService) Update(
	ctx context.Context, input permission.UpdateProductPermissionInput,
) (permission.ProductPermission, error) {
	logger := s.logger.With(slog.String("operation", "Update"))

	if err := validate.ValidateStruct(input); err != nil {
		return permission.ProductPermission{}, err
	}

	existing, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to find permission for update",
			slog.String("product_id", input.ProductID),
			slog.String("name", input.Name),
			log.Err(err),
		)
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
		logger.ErrorContext(ctx, "failed to update permission",
			slog.String("product_id", input.ProductID),
			slog.String("name", input.Name),
			log.Err(err),
		)
		return permission.ProductPermission{}, fault.ErrUnexpected
	}

	logger.InfoContext(ctx, "permission updated",
		slog.String("product_id", input.ProductID),
		slog.String("name", input.Name),
	)

	return result, nil
}

func (s *permissionService) Delete(
	ctx context.Context, input permission.DeleteProductPermissionInput,
) error {
	logger := s.logger.With(slog.String("operation", "Delete"))

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	found, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to check permission existence",
			slog.String("product_id", input.ProductID),
			slog.String("name", input.Name),
			log.Err(err),
		)
		return fault.ErrUnexpected
	}

	if found.IsAbsent() {
		return nil
	}
	exists := found.Value()

	roleCount, err := s.permissionRepo.CountAPIKeyAssignments(ctx, input.ProductID, exists.Name)
	if err != nil {
		logger.ErrorContext(ctx, "failed to count role assignments for permission",
			slog.String("product_id", input.ProductID),
			slog.String("name", input.Name),
			log.Err(err),
		)
		return fault.ErrUnexpected
	}

	if roleCount > 0 {
		return permission.NewPermissionAssignedToAPIKeysError(
			input.ProductID, input.Name, roleCount,
		)
	}

	err = s.permissionRepo.DeleteByID(ctx, input.ProductID, exists.Name)
	if err != nil {
		logger.ErrorContext(ctx, "failed to delete permission",
			slog.String("product_id", input.ProductID),
			slog.String("name", input.Name),
			log.Err(err),
		)
		return fault.ErrUnexpected
	}

	logger.InfoContext(ctx, "permission deleted",
		slog.String("product_id", input.ProductID),
		slog.String("name", input.Name),
	)

	return nil
}

func (s *permissionService) FindByProductAndPermissionName(
	ctx context.Context, input permission.FindProductPermissionInput,
) (permission.ProductPermission, error) {
	logger := s.logger.With(slog.String("operation", "FindByProductAndPermissionName"))

	if err := validate.ValidateStruct(input); err != nil {
		return permission.ProductPermission{}, err
	}

	found, err := s.permissionRepo.FindByProductIDAndPermissionName(
		ctx, input.ProductID, input.Name,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to find permission",
			slog.String("product_id", input.ProductID),
			slog.String("name", input.Name),
			log.Err(err),
		)
		return permission.ProductPermission{}, fault.ErrUnexpected
	}

	return found.ToResult(permission.ErrPermissionNotFound).Value()
}

func (s *permissionService) SearchByProductID(
	ctx context.Context, input permission.SearchProductPermissionInput,
) (search.Result[permission.ProductPermission], error) {
	logger := s.logger.With(slog.String("operation", "SearchByProductID"))

	if err := input.Validate(); err != nil {
		return search.Result[permission.ProductPermission]{}, err
	}

	result, err := s.permissionRepo.SearchByProduct(ctx, input.ProductID, input.Request)
	if err != nil {
		if searchx.IsClientError(err) {
			return search.Result[permission.ProductPermission]{}, err
		}
		logger.ErrorContext(ctx, "failed to search permissions",
			slog.String("product_id", input.ProductID),
			log.Err(err),
		)
		return search.Result[permission.ProductPermission]{}, fault.ErrUnexpected
	}

	return result, nil
}

func (s *permissionService) CheckPermissionExists(
	ctx context.Context, productID string, permissions []string,
) (bool, error) {
	logger := s.logger.With(slog.String("operation", "CheckPermissionExists"))

	if len(permissions) == 0 {
		return false, nil
	}
	perms, err := s.permissionRepo.FindByProductIDAndPermissionNames(
		ctx, productID, permissions,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to check permissions existence",
			slog.String("product_id", productID),
			slog.Int("permission_count", len(permissions)),
			log.Err(err),
		)
		return false, fault.ErrUnexpected
	}
	return len(perms) == len(permissions), nil
}
