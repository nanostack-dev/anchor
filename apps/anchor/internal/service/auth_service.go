package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nanostack-dev/shared/toolkit"

	"anchor/internal/domain/auth"
	"anchor/internal/domain/invitation"
	"anchor/internal/domain/platform"
	"anchor/internal/domain/tenant"
	"anchor/internal/mapper"
	"anchor/internal/repository"
	"anchor/internal/service/config"

	"github.com/rs/zerolog"

	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(ctx context.Context, input auth.RegisterInput) (
		platform.User, error,
	)
	Login(ctx context.Context, input auth.LoginInput) (auth.LoginOutput, error)
	RefreshToken(ctx context.Context, input auth.RefreshTokenInput) (auth.LoginOutput, error)
	GetUserByTenantIDAndID(ctx context.Context, tenantID, userID string) (
		*platform.User, error,
	)
}

type authService struct {
	db                     *sql.DB
	userRepo               repository.UserRepository
	tenantRepo             repository.TenantRepository
	invitationRepo         repository.InvitationRepository
	platformTenantUserRepo repository.PlatformTenantUserRepository
	platformUserMapper     *mapper.PlatformUserMapper
	authCfg                config.AuthConfig
	jwt                    JWTHelper
	logger                 zerolog.Logger
}

func NewAuthService(
	db *sql.DB,
	userRepo repository.UserRepository,
	tenantRepo repository.TenantRepository,
	invitationRepository repository.InvitationRepository,
	platformTenantUserRepo repository.PlatformTenantUserRepository,
	authCfg config.AuthConfig,
	jwtHelper JWTHelper,
	logger zerolog.Logger,
) AuthService {
	return &authService{
		db:                     db,
		userRepo:               userRepo,
		tenantRepo:             tenantRepo,
		invitationRepo:         invitationRepository,
		platformTenantUserRepo: platformTenantUserRepo,
		platformUserMapper:     mapper.NewPlatformUserMapper(),
		authCfg:                authCfg,
		jwt:                    jwtHelper,
		logger:                 logger.With().Str("component", "auth_service").Logger(),
	}
}

func (s *authService) GetUserByTenantIDAndID(
	ctx context.Context, tenantID, userID string,
) (*platform.User, error) {
	logger := s.logger.With().Str("operation", "GetUserByTenantIDAndID").Logger()

	user, err := s.platformTenantUserRepo.FindByTenantIDAndUserID(ctx, tenantID, userID, nil)
	if err != nil {
		logger.Error().
			Str("tenant_id", tenantID).
			Str("user_id", userID).
			Err(err).
			Msg("failed to find user by tenant and user ID")
		return nil, err
	}
	return user, nil
}

func (s *authService) Register(
	ctx context.Context, input auth.RegisterInput,
) (platform.User, error) {
	logger := s.logger.With().Str("operation", "Register").Logger()

	logger.Info().Str("email", input.Email).Msg("registering platform user")
	if validationErr := toolkit.ValidateStruct(input); validationErr != nil {
		logger.Warn().Err(validationErr).Msg("registration input validation failed")
		return platform.User{}, validationErr
	}
	return toolkit.WithTxReturn(
		s.db, func(tx *sql.Tx) (platform.User, error) {
			optUserByEmail, err := s.userRepo.FindByEmail(
				ctx, input.Email, &toolkit.DBOptions{Tx: tx},
			)
			if err != nil {
				logger.Error().
					Str("email", input.Email).
					Err(err).
					Msg("failed to find user by email")
				return platform.User{}, fmt.Errorf("failed during user lookup: %w", err)
			}

			if optUserByEmail != nil {
				logger.Warn().Str("email", input.Email).Msg("user already exists")
				return platform.User{}, ErrUserAlreadyExists
			}
			currentTenantID, role, err := s.setupTenantForRegistration(
				ctx, tx, input.InvitationCode, input.Email, input.TenantName, logger,
			)
			if err != nil {
				return platform.User{}, err
			}

			hashedPassword, err := bcrypt.GenerateFromPassword(
				[]byte(input.Password), bcrypt.DefaultCost,
			)
			if err != nil {
				logger.Error().Err(err).Msg("failed to hash password")
				return platform.User{}, fmt.Errorf("failed to hash password: %w", err)
			}

			newUser := auth.User{
				Email:          input.Email,
				HashedPassword: string(hashedPassword),
			}
			newUser.GenerateID()

			newUser, err = s.userRepo.Create(ctx, newUser, &toolkit.DBOptions{Tx: tx})
			if err != nil {
				logger.Error().Str("email", input.Email).Err(err).Msg("failed to create user")
				return platform.User{}, fmt.Errorf("failed to create user: %w", err)
			}

			// Create the unified platform user record using the repository
			platformUser := platform.User{
				UserID:           newUser.ID,
				ExternalID:       newUser.ExternalID,
				Name:             newUser.Name,
				Email:            newUser.Email,
				HashedPassword:   newUser.HashedPassword,
				CreatedAt:        newUser.CreatedAt,
				UpdatedAt:        newUser.UpdatedAt,
				PlatformTenantID: currentTenantID,
				Role:             role,
			}
			platformUser.GenerateID()

			// Use the repository Create method to persist the platform user
			createdPlatformUser, err := s.platformTenantUserRepo.Create(
				ctx, platformUser, &toolkit.DBOptions{Tx: tx},
			)
			if err != nil {
				logger.Error().Str("email", input.Email).Err(err).Msg("failed to create platform user")
				return platform.User{}, fmt.Errorf(
					"failed to create platform user: %w", err,
				)
			}

			logger.Info().
				Str("user_id", createdPlatformUser.UserID).
				Str("platform_user_id", createdPlatformUser.ID).
				Str("email", input.Email).
				Msg("platform user registered successfully")

			return createdPlatformUser, nil
		},
	)
}

func (s *authService) handleInvitation(
	ctx context.Context, tx *sql.Tx, email string, code string, logger zerolog.Logger,
) (invitation.PlatformInvitation, error) {
	if code == "" {
		logger.Error().Msg("invitation code is empty")
		return invitation.PlatformInvitation{}, ErrInvitationCodeNotProvided
	}
	optInvitation, err := s.invitationRepo.FindByCodeAndEmail(
		ctx, code, email, &toolkit.DBOptions{Tx: tx},
	)
	if err != nil {
		logger.Error().Str("email", email).Err(err).Msg("failed to find invitation")
		return invitation.PlatformInvitation{}, fmt.Errorf("failed to find invitation: %w", err)
	}
	if optInvitation == nil {
		logger.Error().Str("email", email).Msg("invitation not found")
		return invitation.PlatformInvitation{}, ErrInvitationCodeIsInvalid
	}
	// then delete the invitation because it is used
	if err = s.invitationRepo.DeleteByTenantIDAndID(
		ctx, optInvitation.PlatformTenantID, optInvitation.ID, &toolkit.DBOptions{Tx: tx},
	); err != nil {
		logger.Error().
			Str("invitation_id", optInvitation.ID).
			Str("tenant_id", optInvitation.PlatformTenantID).
			Err(err).
			Msg("failed to delete invitation")
		return invitation.PlatformInvitation{}, fmt.Errorf("failed to delete invitation: %w", err)
	}
	return *optInvitation, nil
}

func (s *authService) Login(
	ctx context.Context, input auth.LoginInput,
) (auth.LoginOutput, error) {
	logger := s.logger.With().Str("operation", "Login").Logger()

	if validationErr := toolkit.ValidateStruct(input); validationErr != nil {
		return auth.LoginOutput{}, validationErr
	}

	user, err := s.userRepo.FindByEmail(ctx, input.Email, nil)
	if err != nil {
		logger.Error().Str("email", input.Email).Err(err).Msg("failed to find user by email")
		return auth.LoginOutput{}, fmt.Errorf("failed during user lookup: %w", err)
	}
	if user == nil {
		return auth.LoginOutput{}, ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(input.Password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return auth.LoginOutput{}, ErrInvalidCredentials
		}
		logger.Error().Str("user_id", user.ID).Err(err).Msg("failed to compare password hash")
		return auth.LoginOutput{}, fmt.Errorf("failed during password comparison: %w", err)
	}

	// For login, we need to find the platform user by email first to get the tenant Name
	// Since we only have the basic user info, we need to search across all tenants
	// This is a temporary solution - ideally login should include tenant context

	// Get the first tenant (for now assuming single tenant setup)
	tenants, err := s.tenantRepo.FindAll(ctx, nil)
	if err != nil || len(tenants) == 0 {
		logger.Error().Err(err).Msg("no tenants found")
		return auth.LoginOutput{}, errors.New("no tenant configuration found")
	}

	// Try to find platform user in the first tenant
	platformUser, err := s.platformTenantUserRepo.FindByTenantIDAndEmail(
		ctx, tenants[0].ID, user.Email, nil,
	)
	if err != nil {
		return auth.LoginOutput{}, err
	}
	if platformUser == nil {
		logger.Error().Str("user_id", user.ID).Msg("platform user not found")
		return auth.LoginOutput{}, errors.New("platform user not found")
	}

	accessToken, refreshToken, err := s.jwt.GenerateTokens(
		user.ID, platformUser.PlatformTenantID,
	)
	if err != nil {
		logger.Error().Str("user_id", user.ID).Err(err).Msg("failed to generate tokens")
		return auth.LoginOutput{}, toolkit.ErrUnexpected
	}

	logger.Info().Str("user_id", user.ID).Msg("user logged in successfully")

	user.HashedPassword = ""
	return auth.LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *authService) RefreshToken(
	ctx context.Context, input auth.RefreshTokenInput,
) (auth.LoginOutput, error) {
	logger := s.logger.With().Str("operation", "RefreshToken").Logger()

	if validationErr := toolkit.ValidateStruct(input); validationErr != nil {
		return auth.LoginOutput{}, validationErr
	}

	claims, err := s.jwt.ValidateRefreshToken(input.RefreshToken)
	if err != nil {
		// Don't log the token itself, but log the failure
		logger.Warn().Err(err).Msg("refresh token validation failed")
		return auth.LoginOutput{}, ErrTokenRefreshFailed
	}

	newAccessToken, newRefreshToken, err := s.jwt.GenerateTokens(claims.UserID, claims.TenantID)
	if err != nil {
		logger.Error().
			Str("user_id", claims.UserID).
			Err(err).
			Msg("failed to generate new tokens during refresh")
		return auth.LoginOutput{}, toolkit.ErrUnexpected
	}

	user, err := s.platformTenantUserRepo.FindByTenantIDAndUserID(
		ctx, claims.TenantID, claims.UserID, nil,
	)
	if err != nil {
		logger.Error().
			Str("user_id", claims.UserID).
			Err(err).
			Msg("failed to find user during token refresh")
		return auth.LoginOutput{}, fmt.Errorf(
			"failed to retrieve user details during refresh: %w", err,
		)
	}
	if user == nil {
		return auth.LoginOutput{}, ErrUserNotFound
	}

	logger.Debug().Str("user_id", claims.UserID).Msg("token refreshed successfully")

	user.HashedPassword = ""
	return auth.LoginOutput{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *authService) setupTenantForRegistration(
	ctx context.Context, tx *sql.Tx, invitationCode *string, email string, tenantName *string,
	logger zerolog.Logger,
) (string, platform.TenantRole, error) {
	count, err := s.tenantRepo.Count(ctx, &toolkit.DBOptions{Tx: tx})
	if err != nil {
		logger.Error().Err(err).Msg("failed to count tenants")
		return "", "", fmt.Errorf("failed to count tenants: %w", err)
	}

	role := platform.TenantRoleOwner
	if count > 0 {
		role = platform.TenantRoleAdmin
	}

	if count == 0 {
		logger.Warn().Msg("no tenants found, creating one")

		// Use provided tenant name or default to "Default"
		tenantNameToUse := "Default"
		if tenantName != nil && *tenantName != "" {
			tenantNameToUse = *tenantName
		}

		t := tenant.PlatformTenant{
			Name:   tenantNameToUse,
			Status: tenant.Active,
		}
		t.GenerateID()
		currentTenant, createErr := s.tenantRepo.Create(
			ctx,
			t, &toolkit.DBOptions{Tx: tx},
		)
		if createErr != nil {
			logger.Error().Str("tenant_name", tenantNameToUse).Err(createErr).Msg("failed to create tenant")
			return "", "", fmt.Errorf("failed to create tenant: %w", createErr)
		}
		logger.Info().Str("tenant_id", currentTenant.ID).Str("tenant_name", tenantNameToUse).Msg("tenant created")
		return currentTenant.ID, role, nil
	}

	if invitationCode == nil {
		logger.Warn().Msg("invitation code is required for existing tenants")
		return "", "", ErrInvitationCodeNotProvided
	}

	logger.Info().Msg("using invitation code for registration")
	userInvitation, inviteErr := s.handleInvitation(ctx, tx, email, *invitationCode, logger)
	if inviteErr != nil {
		logger.Error().Err(inviteErr).Msg("failed to handle invitation")
		return "", "", fmt.Errorf("failed to handle invitation: %w", inviteErr)
	}
	return userInvitation.PlatformTenantID, role, nil
}
