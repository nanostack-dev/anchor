package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/domain/auth"
	"anchor/internal/domain/invitation"
	"anchor/internal/domain/platform"
	"anchor/internal/domain/platformsession"
	"anchor/internal/domain/tenant"
	"anchor/internal/mapper"
	"anchor/internal/repository"
	"anchor/internal/security"
	"anchor/internal/service/config"
	sessionrepository "anchor/internal/session/repository"

	"github.com/rs/zerolog"

	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(ctx context.Context, input auth.RegisterInput) (
		platform.User, error,
	)
	Login(ctx context.Context, input auth.LoginInput) (auth.LoginOutput, error)
	RefreshToken(ctx context.Context, input auth.RefreshTokenInput) (auth.LoginOutput, error)
	Logout(ctx context.Context, input auth.LogoutInput) error
	// StartSession opens a session for a platform user and issues its first
	// token pair. It checks no credential: the caller must already have
	// authenticated the user.
	StartSession(ctx context.Context, input auth.StartSessionInput) (auth.LoginOutput, error)
	GetUserByTenantIDAndID(ctx context.Context, tenantID, userID string) (
		*platform.User, error,
	)
}

type authService struct {
	transactor             transactor.Transactor
	userRepo               repository.UserRepository
	tenantRepo             repository.TenantRepository
	invitationRepo         repository.InvitationRepository
	platformTenantUserRepo repository.PlatformTenantUserRepository
	sessionRepo            sessionrepository.Repository
	platformUserMapper     *mapper.PlatformUserMapper
	authCfg                config.AuthConfig
	jwt                    JWTHelper
	logger                 zerolog.Logger
}

func NewAuthService(
	transactor transactor.Transactor,
	userRepo repository.UserRepository,
	tenantRepo repository.TenantRepository,
	invitationRepository repository.InvitationRepository,
	platformTenantUserRepo repository.PlatformTenantUserRepository,
	sessionRepo sessionrepository.Repository,
	authCfg config.AuthConfig,
	jwtHelper JWTHelper,
	logger zerolog.Logger,
) AuthService {
	return &authService{
		transactor:             transactor,
		userRepo:               userRepo,
		tenantRepo:             tenantRepo,
		invitationRepo:         invitationRepository,
		platformTenantUserRepo: platformTenantUserRepo,
		sessionRepo:            sessionRepo,
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

	found, err := s.platformTenantUserRepo.FindByTenantIDAndUserID(ctx, tenantID, userID)
	if err != nil {
		logger.Error().
			Str("tenant_id", tenantID).
			Str("user_id", userID).
			Err(err).
			Msg("failed to find user by tenant and user ID")
		return nil, err
	}
	return found.ToPtr(), nil
}

func (s *authService) Register(
	ctx context.Context, input auth.RegisterInput,
) (platform.User, error) {
	logger := s.logger.With().Str("operation", "Register").Logger()

	logger.Info().Str("email", input.Email).Msg("registering platform user")
	if validationErr := validate.ValidateStruct(input); validationErr != nil {
		logServiceError(logger, validationErr).Msg("registration input validation failed")
		return platform.User{}, validationErr
	}

	var createdPlatformUser platform.User
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		// Resolve the tenant and validate the invitation code before reading the
		// user table. On this unauthenticated endpoint, checking email existence
		// first leaked whether an address already had an account: an invalid or
		// missing invitation code returned 400 for an unknown email but the email
		// check returned 409 for a known one, giving a pre-auth enumeration oracle.
		// Validating the invitation first makes both cases return the same
		// INVITATION_CODE_* response, so a caller without a valid code cannot tell
		// the two apart.
		currentTenantID, role, err := s.setupTenantForRegistration(
			txCtx, input.InvitationCode, input.Email, input.TenantName, logger,
		)
		if err != nil {
			return err
		}

		foundUserByEmail, err := s.userRepo.FindByEmail(txCtx, input.Email)
		if err != nil {
			logger.Error().
				Str("email", input.Email).
				Err(err).
				Msg("failed to find user by email")
			return fmt.Errorf("failed during user lookup: %w", err)
		}

		if foundUserByEmail.IsPresent() {
			logger.Debug().Str("email", input.Email).Msg("user already exists")
			return ErrUserAlreadyExists
		}

		hashedPassword, err := bcrypt.GenerateFromPassword(
			[]byte(input.Password), bcrypt.DefaultCost,
		)
		if err != nil {
			logger.Error().Err(err).Msg("failed to hash password")
			return fmt.Errorf("failed to hash password: %w", err)
		}

		newUser := auth.User{
			Email:          input.Email,
			HashedPassword: string(hashedPassword),
		}
		newUser.GenerateID()

		newUser, err = s.userRepo.Create(txCtx, newUser)
		if err != nil {
			logger.Error().Str("email", input.Email).Err(err).Msg("failed to create user")
			return fmt.Errorf("failed to create user: %w", err)
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
		resUser, err := s.platformTenantUserRepo.Create(txCtx, platformUser)
		if err != nil {
			logger.Error().Str("email", input.Email).Err(err).Msg("failed to create platform user")
			return fmt.Errorf("failed to create platform user: %w", err)
		}

		createdPlatformUser = resUser
		logger.Info().
			Str("user_id", createdPlatformUser.UserID).
			Str("platform_user_id", createdPlatformUser.ID).
			Str("email", input.Email).
			Msg("platform user registered successfully")

		return nil
	})

	return createdPlatformUser, err
}

func (s *authService) handleInvitation(
	ctx context.Context, email string, code string, logger zerolog.Logger,
) (invitation.PlatformInvitation, error) {
	if code == "" {
		logger.Debug().Msg("invitation code is empty")
		return invitation.PlatformInvitation{}, ErrInvitationCodeNotProvided
	}
	foundInvitation, err := s.invitationRepo.FindByCodeAndEmail(ctx, code, email)
	if err != nil {
		logger.Error().Str("email", email).Err(err).Msg("failed to find invitation")
		return invitation.PlatformInvitation{}, fmt.Errorf("failed to find invitation: %w", err)
	}
	if foundInvitation.IsAbsent() {
		logger.Debug().Str("email", email).Msg("invitation not found")
		return invitation.PlatformInvitation{}, ErrInvitationCodeIsInvalid
	}
	optInvitation := foundInvitation.Value()
	// then delete the invitation because it is used
	if err = s.invitationRepo.DeleteByTenantIDAndID(
		ctx, optInvitation.PlatformTenantID, optInvitation.ID,
	); err != nil {
		logger.Error().
			Str("invitation_id", optInvitation.ID).
			Str("tenant_id", optInvitation.PlatformTenantID).
			Err(err).
			Msg("failed to delete invitation")
		return invitation.PlatformInvitation{}, fmt.Errorf("failed to delete invitation: %w", err)
	}
	return optInvitation, nil
}

func (s *authService) Login(
	ctx context.Context, input auth.LoginInput,
) (auth.LoginOutput, error) {
	logger := s.logger.With().Str("operation", "Login").Logger()

	if validationErr := validate.ValidateStruct(input); validationErr != nil {
		return auth.LoginOutput{}, validationErr
	}

	foundUser, err := s.userRepo.FindByEmail(ctx, input.Email)
	if err != nil {
		logger.Error().Str("email", input.Email).Err(err).Msg("failed to find user by email")
		return auth.LoginOutput{}, fmt.Errorf("failed during user lookup: %w", err)
	}
	if foundUser.IsAbsent() {
		return auth.LoginOutput{}, ErrInvalidCredentials
	}
	user := foundUser.ToPtr()

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
	tenants, err := s.tenantRepo.FindAll(ctx)
	if err != nil || len(tenants) == 0 {
		logger.Error().Err(err).Msg("no tenants found")
		return auth.LoginOutput{}, errors.New("no tenant configuration found")
	}

	// Try to find platform user in the first tenant
	foundPlatformUser, err := s.platformTenantUserRepo.FindByTenantIDAndEmail(
		ctx, tenants[0].ID, user.Email,
	)
	if err != nil {
		return auth.LoginOutput{}, err
	}
	if foundPlatformUser.IsAbsent() {
		// A valid email/password whose user has no platform-tenant membership is
		// an authentication failure, not a server error. Return the same modelled
		// credential error as the sibling branches above so the strict handler
		// responds 400 (logged at info) instead of collapsing an unmodelled error
		// into a 500 logged at error. Warn keeps the data-consistency signal
		// visible (the user exists but is not provisioned) without the false
		// server-error alarm, and mirroring the other branches avoids leaking
		// whether an address is provisioned.
		logger.Warn().Str("user_id", user.ID).Msg("platform user not found for authenticated user")
		return auth.LoginOutput{}, ErrInvalidCredentials
	}
	platformUser := foundPlatformUser.Value()

	output, err := s.StartSession(ctx, auth.StartSessionInput{
		PlatformUserID: platformUser.ID,
		UserID:         user.ID,
		TenantID:       platformUser.PlatformTenantID,
	})
	if err != nil {
		logger.Error().Str("user_id", user.ID).Err(err).Msg("failed to start session")
		return auth.LoginOutput{}, fault.ErrUnexpected
	}

	logger.Info().Str("user_id", user.ID).Msg("user logged in successfully")
	return output, nil
}

func (s *authService) StartSession(
	ctx context.Context, input auth.StartSessionInput,
) (auth.LoginOutput, error) {
	if validationErr := validate.ValidateStruct(input); validationErr != nil {
		return auth.LoginOutput{}, validationErr
	}

	now := time.Now()
	session := platformsession.Session{PlatformUserID: input.PlatformUserID}
	session.GenerateID()
	issued, refreshToken, err := s.issueTokens(input.UserID, input.TenantID, session.ID)
	if err != nil {
		return auth.LoginOutput{}, err
	}
	session.ExpiresAt = refreshToken.ExpiresAt

	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if pruneErr := s.sessionRepo.DeleteExpiredSessions(txCtx, input.PlatformUserID, now); pruneErr != nil {
			return fmt.Errorf("failed to prune expired sessions: %w", pruneErr)
		}
		if _, createErr := s.sessionRepo.CreateSession(txCtx, session); createErr != nil {
			return fmt.Errorf("failed to create session: %w", createErr)
		}
		if _, storeErr := s.sessionRepo.CreateRefreshToken(txCtx, refreshToken); storeErr != nil {
			return fmt.Errorf("failed to store refresh token: %w", storeErr)
		}
		return nil
	})
	if err != nil {
		return auth.LoginOutput{}, err
	}

	return auth.LoginOutput{AccessToken: issued.AccessToken, RefreshToken: issued.RefreshToken}, nil
}

// issueTokens signs a new token pair for the session, and returns the refresh
// token's row with only its hash.
func (s *authService) issueTokens(
	userID, tenantID, sessionID string,
) (IssuedTokens, platformsession.RefreshToken, error) {
	refreshToken := platformsession.RefreshToken{SessionID: sessionID}
	refreshToken.GenerateID()
	issued, err := s.jwt.GenerateTokens(TokenSubject{
		UserID:         userID,
		TenantID:       tenantID,
		SessionID:      sessionID,
		RefreshTokenID: refreshToken.ID,
	})
	if err != nil {
		return IssuedTokens{}, platformsession.RefreshToken{}, fmt.Errorf("failed to generate tokens: %w", err)
	}
	refreshToken.TokenHash = security.HashSecret(issued.RefreshToken)
	refreshToken.ExpiresAt = issued.RefreshExpiresAt
	return issued, refreshToken, nil
}

func (s *authService) RefreshToken(
	ctx context.Context, input auth.RefreshTokenInput,
) (auth.LoginOutput, error) {
	logger := s.logger.With().Str("operation", "RefreshToken").Logger()

	if validationErr := validate.ValidateStruct(input); validationErr != nil {
		return auth.LoginOutput{}, validationErr
	}

	claims, err := s.jwt.ValidateRefreshToken(input.RefreshToken)
	if err != nil {
		// Don't log the token itself, but log the failure
		logger.Debug().Err(err).Msg("refresh token validation failed")
		return auth.LoginOutput{}, ErrTokenRefreshFailed
	}
	if claims.SessionID == "" {
		logger.Debug().Str("user_id", claims.UserID).Msg("refresh token predates sessions")
		return auth.LoginOutput{}, ErrTokenRefreshFailed
	}

	now := time.Now()
	var output auth.LoginOutput
	reuseDetected := false
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		foundSession, lockErr := s.sessionRepo.LockSession(txCtx, claims.SessionID)
		if lockErr != nil {
			return fmt.Errorf("failed to lock session: %w", lockErr)
		}
		if foundSession.IsAbsent() {
			return ErrTokenRefreshFailed
		}
		session := foundSession.Value()
		if session.IsRevoked() {
			return ErrTokenRefreshFailed
		}

		foundToken, findErr := s.sessionRepo.FindRefreshToken(
			txCtx, session.ID, security.HashSecret(input.RefreshToken),
		)
		if findErr != nil {
			return fmt.Errorf("failed to find refresh token: %w", findErr)
		}
		if foundToken.IsAbsent() {
			return ErrTokenRefreshFailed
		}
		presented := foundToken.Value()

		switch presented.ExchangeAt(now) {
		case platformsession.ExchangeReuse:
			reuseDetected = true
			return s.sessionRepo.RevokeSession(txCtx, session.ID, now)
		case platformsession.ExchangeRotate:
			if rotateErr := s.sessionRepo.MarkRefreshTokenRotated(txCtx, presented.ID, now); rotateErr != nil {
				return fmt.Errorf("failed to rotate refresh token: %w", rotateErr)
			}
		case platformsession.ExchangeWithinGrace:
		}

		if pruneErr := s.sessionRepo.DeleteExpiredRefreshTokens(txCtx, session.ID, now); pruneErr != nil {
			return fmt.Errorf("failed to prune expired refresh tokens: %w", pruneErr)
		}
		issued, refreshToken, issueErr := s.issueTokens(claims.UserID, claims.TenantID, session.ID)
		if issueErr != nil {
			return issueErr
		}
		if _, storeErr := s.sessionRepo.CreateRefreshToken(txCtx, refreshToken); storeErr != nil {
			return fmt.Errorf("failed to store refresh token: %w", storeErr)
		}
		if extendErr := s.sessionRepo.ExtendSession(txCtx, session.ID, refreshToken.ExpiresAt); extendErr != nil {
			return fmt.Errorf("failed to extend session: %w", extendErr)
		}
		output = auth.LoginOutput{AccessToken: issued.AccessToken, RefreshToken: issued.RefreshToken}
		return nil
	})
	if err != nil {
		if !errors.Is(err, ErrTokenRefreshFailed) {
			logger.Error().Str("user_id", claims.UserID).Err(err).Msg("failed to refresh token")
		}
		return auth.LoginOutput{}, err
	}
	if reuseDetected {
		logger.Warn().
			Str("user_id", claims.UserID).
			Str("session_id", claims.SessionID).
			Msg("rotated refresh token presented again; session revoked")
		return auth.LoginOutput{}, ErrTokenRefreshFailed
	}

	logger.Debug().Str("user_id", claims.UserID).Msg("token refreshed successfully")
	return output, nil
}

func (s *authService) Logout(ctx context.Context, input auth.LogoutInput) error {
	logger := s.logger.With().Str("operation", "Logout").Logger()

	if validationErr := validate.ValidateStruct(input); validationErr != nil {
		return validationErr
	}
	if input.AccessToken == "" {
		return nil
	}

	claims, err := s.jwt.VerifyAccessTokenIgnoringExpiry(input.AccessToken)
	if err != nil {
		logger.Debug().Err(err).Msg("logout access token rejected; nothing to revoke")
		return nil
	}
	if claims.SessionID == "" {
		return nil
	}

	if revokeErr := s.sessionRepo.RevokeSession(ctx, claims.SessionID, time.Now()); revokeErr != nil {
		logger.Error().Str("user_id", claims.UserID).Err(revokeErr).Msg("failed to revoke session")
		return revokeErr
	}
	logger.Info().Str("user_id", claims.UserID).Str("session_id", claims.SessionID).Msg("session revoked")
	return nil
}

func (s *authService) setupTenantForRegistration(
	ctx context.Context, invitationCode *string, email string, tenantName *string,
	logger zerolog.Logger,
) (string, platform.TenantRole, error) {
	count, err := s.tenantRepo.Count(ctx)
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
		currentTenant, createErr := s.tenantRepo.Create(ctx, t)
		if createErr != nil {
			logger.Error().Str("tenant_name", tenantNameToUse).Err(createErr).Msg("failed to create tenant")
			return "", "", fmt.Errorf("failed to create tenant: %w", createErr)
		}
		logger.Info().Str("tenant_id", currentTenant.ID).Str("tenant_name", tenantNameToUse).Msg("tenant created")
		return currentTenant.ID, role, nil
	}

	if invitationCode == nil {
		logger.Debug().Msg("invitation code is required for existing tenants")
		return "", "", ErrInvitationCodeNotProvided
	}

	logger.Info().Msg("using invitation code for registration")
	userInvitation, inviteErr := s.handleInvitation(ctx, email, *invitationCode, logger)
	if inviteErr != nil {
		logServiceError(logger, inviteErr).Msg("failed to handle invitation")
		return "", "", fmt.Errorf("failed to handle invitation: %w", inviteErr)
	}
	return userInvitation.PlatformTenantID, role, nil
}
