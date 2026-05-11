package service

import (
	"context"
	"net/http"

	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

	"anchor/internal/domain/platform"
	"anchor/internal/repository"
	"anchor/internal/security"

	"github.com/rs/zerolog"
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
	logger           zerolog.Logger
}

func NewPlatformUserService(
	platformUserRepo repository.PlatformTenantUserRepository,
	logger zerolog.Logger,
) PlatformUserService {
	return &platformUserService{
		platformUserRepo: platformUserRepo,
		logger:           logger.With().Str("component", "platform_user_service").Logger(),
	}
}

func (s *platformUserService) SearchPlatformUsers(
	ctx context.Context, input platform.SearchPlatformUsersInput,
) (search.Result[platform.User], error) {
	logger := s.logger.With().Str("operation", "SearchPlatformUsers").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return search.Result[platform.User]{}, err
	}
	result, err := s.platformUserRepo.SearchByTenantID(ctx, input.TenantID, input.Request, nil)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Err(err).
			Msg("failed to search platform users")
		return search.Result[platform.User]{}, err
	}
	return result, nil
}

func (s *platformUserService) GetPlatformUserByUserID(
	ctx context.Context, input platform.GetPlatformUserByUserIDInput,
) (*platform.User, error) {
	logger := s.logger.With().Str("operation", "GetPlatformUserByUserID").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}
	user, err := s.platformUserRepo.FindByTenantIDAndUserID(ctx, input.TenantID, input.UserID, nil)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("user_id", input.UserID).
			Err(err).
			Msg("failed to find platform user by user ID")
		return nil, err
	}
	return user, nil
}

func (s *platformUserService) GetPlatformUser(
	ctx context.Context, input platform.GetPlatformUserInput,
) (*platform.User, error) {
	logger := s.logger.With().Str("operation", "GetPlatformUser").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}
	user, err := s.platformUserRepo.FindByTenantIDAndID(ctx, input.TenantID, input.PlatformUserID, nil)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("platform_user_id", input.PlatformUserID).
			Err(err).
			Msg("failed to find platform user")
		return nil, err
	}
	return user, nil
}

func (s *platformUserService) DeletePlatformUser(
	ctx context.Context, input platform.DeletePlatformUserInput,
) error {
	logger := s.logger.With().Str("operation", "DeletePlatformUser").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
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

	// Prevent self-deletion
	if currentPlatformUser != nil && currentPlatformUser.ID == input.PlatformUserID {
		return toolkit.NewNanostackErrorsWithStatus(
			"SELF_DELETION_NOT_ALLOWED",
			"You cannot delete yourself",
			http.StatusBadRequest,
		)
	}

	err = s.platformUserRepo.DeleteByID(ctx, input.TenantID, input.PlatformUserID, nil)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("platform_user_id", input.PlatformUserID).
			Err(err).
			Msg("failed to delete platform user")
		return err
	}

	logger.Info().
		Str("tenant_id", input.TenantID).
		Str("platform_user_id", input.PlatformUserID).
		Msg("platform user deleted")

	return nil
}
