package service

import (
	"context"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/domain/platform"
	"anchor/internal/repository"
	"anchor/internal/searchx"
	"anchor/internal/security"
)

type PlatformUserService interface {
	SearchPlatformUsers(
		ctx context.Context, input platform.SearchPlatformUsersInput,
	) (search.Result[platform.User], error)
	GetPlatformUser(
		ctx context.Context, input platform.GetPlatformUserInput,
	) (*platform.User, error)
	GetPlatformUserByUserID(
		ctx context.Context, input platform.GetPlatformUserByUserIDInput,
	) (*platform.User, error)
	DeletePlatformUser(
		ctx context.Context, input platform.DeletePlatformUserInput,
	) error
}

type platformUserService struct {
	platformUserRepo repository.PlatformTenantUserRepository
	logger           *slog.Logger
}

func NewPlatformUserService(
	platformUserRepo repository.PlatformTenantUserRepository,
	logger *slog.Logger,
) PlatformUserService {
	return &platformUserService{
		platformUserRepo: platformUserRepo,
		logger:           logger.With(slog.String("component", "platform_user_service")),
	}
}

func (s *platformUserService) SearchPlatformUsers(
	ctx context.Context, input platform.SearchPlatformUsersInput,
) (search.Result[platform.User], error) {
	logger := s.logger.With(slog.String("operation", "SearchPlatformUsers"))

	if err := input.Validate(); err != nil {
		return search.Result[platform.User]{}, err
	}
	result, err := s.platformUserRepo.SearchByTenantID(ctx, input.TenantID, input.Request)
	if err != nil {
		if !searchx.IsClientError(err) {
			logger.ErrorContext(ctx, "failed to search platform users",
				slog.String("tenant_id", input.TenantID),
				log.Err(err),
			)
		}
		return search.Result[platform.User]{}, err
	}
	return result, nil
}

func (s *platformUserService) GetPlatformUserByUserID(
	ctx context.Context, input platform.GetPlatformUserByUserIDInput,
) (*platform.User, error) {
	logger := s.logger.With(slog.String("operation", "GetPlatformUserByUserID"))

	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}
	found, err := s.platformUserRepo.FindByPlatformTenantIDAndUserID(ctx, input.TenantID, input.UserID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to find platform user by user ID",
			slog.String("tenant_id", input.TenantID),
			slog.String("user_id", input.UserID),
			log.Err(err),
		)
		return nil, err
	}
	return found.ToPtr(), nil
}

func (s *platformUserService) GetPlatformUser(
	ctx context.Context, input platform.GetPlatformUserInput,
) (*platform.User, error) {
	logger := s.logger.With(slog.String("operation", "GetPlatformUser"))

	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}
	found, err := s.platformUserRepo.FindByPlatformTenantIDAndID(ctx, input.TenantID, input.PlatformUserID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to find platform user",
			slog.String("tenant_id", input.TenantID),
			slog.String("platform_user_id", input.PlatformUserID),
			log.Err(err),
		)
		return nil, err
	}
	return found.ToPtr(), nil
}

func (s *platformUserService) DeletePlatformUser(
	ctx context.Context, input platform.DeletePlatformUserInput,
) error {
	logger := s.logger.With(slog.String("operation", "DeletePlatformUser"))

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}
	//TODO: we should add member id in the jwt
	userID, err := security.GetCurrentUserID(ctx)
	if err != nil {
		return err
	}
	// Get the current user to prevent self-deletion
	currentPlatformUser, err := s.GetPlatformUserByUserID(
		ctx, platform.GetPlatformUserByUserIDInput{
			TenantID: input.TenantID,
			UserID:   userID,
		},
	)
	if err != nil {
		return err
	}

	if currentPlatformUser != nil && currentPlatformUser.ID == input.PlatformUserID {
		return fault.Forbidden(
			"SELF_DELETION_NOT_ALLOWED",
			"You cannot delete your own platform user account.",
		)
	}

	target, err := s.platformUserRepo.FindByPlatformTenantIDAndID(ctx, input.TenantID, input.PlatformUserID)
	if err != nil {
		return err
	}
	callerIsOwner := currentPlatformUser != nil && currentPlatformUser.Role == platform.TenantRoleOwner
	if target.IsPresent() && target.Value().Role == platform.TenantRoleOwner && !callerIsOwner {
		return fault.Forbidden("OWNER_DELETION_NOT_ALLOWED", "Only an owner can delete the tenant owner.")
	}

	err = s.platformUserRepo.DeleteByPlatformTenantIDAndID(ctx, input.TenantID, input.PlatformUserID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to delete platform user",
			slog.String("tenant_id", input.TenantID),
			slog.String("platform_user_id", input.PlatformUserID),
			log.Err(err),
		)
		return err
	}

	logger.InfoContext(ctx, "platform user deleted",
		slog.String("tenant_id", input.TenantID),
		slog.String("platform_user_id", input.PlatformUserID),
	)

	return nil
}
