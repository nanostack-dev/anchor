package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/invitation"
	"anchor/internal/domain/platform"
	"anchor/internal/mapper"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
)

const invitationCodeLength = 16

type InvitationService interface {
	CreateInvitation(
		ctx context.Context, input invitation.CreateInvitationInput,
	) (invitation.PlatformInvitation, error)
	DeleteInvitation(
		ctx context.Context, input invitation.DeleteInvitationInput,
	) error
	SearchInvitation(
		ctx context.Context, input invitation.SearchInvitationInput,
	) (search.Result[invitation.PlatformInvitation], error)
}

type invitationService struct {
	invitationRepo   repository.InvitationRepository
	platformUserRepo repository.PlatformTenantUserRepository
	logger           zerolog.Logger
	mapper           *mapper.InvitationMapper
}

func NewInvitationService(
	platformUserRepo repository.PlatformTenantUserRepository,
	invitationRepo repository.InvitationRepository, logger zerolog.Logger,
) InvitationService {
	return &invitationService{
		invitationRepo:   invitationRepo,
		logger:           logger.With().Str("component", "invitation_service").Logger(),
		mapper:           mapper.NewInvitationMapper(),
		platformUserRepo: platformUserRepo,
	}
}

func (s *invitationService) CreateInvitation(
	ctx context.Context, input invitation.CreateInvitationInput,
) (invitation.PlatformInvitation, error) {
	logger := s.logger.With().Str("operation", "CreateInvitation").Logger()

	if err := validateStruct(input); err != nil {
		return invitation.PlatformInvitation{}, err
	}
	foundPlatformUser, err := s.platformUserRepo.FindByTenantIDAndEmail(
		ctx, input.TenantID, input.Email,
	)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("email", input.Email).
			Err(err).
			Msg("failed to find user by email")
		return invitation.PlatformInvitation{}, fault.ErrUnexpected
	}
	if foundPlatformUser.IsPresent() {
		logger.Debug().
			Str("tenant_id", input.TenantID).
			Str("email", input.Email).
			Msg("user already exists")
		// A uniqueness collision: this address already has a user account.
		// Inviting a different address succeeds, so this is a conflict, not a
		// bad request.
		return invitation.PlatformInvitation{}, fault.Conflict(
			"INVITATION_USER_ALREADY_EXISTS",
			"This email address already has a user account. "+
				"Check if the person is already a member, or invite a different email address.",
		)
	}
	foundInvitation, err := s.invitationRepo.FindByTenantIDAndEmail(
		ctx, input.TenantID, input.Email,
	)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("email", input.Email).
			Err(err).
			Msg("failed to find invitation by email")
		return invitation.PlatformInvitation{}, fault.ErrUnexpected
	}
	if foundInvitation.IsPresent() {
		logger.Debug().
			Str("tenant_id", input.TenantID).
			Str("email", input.Email).
			Msg("invitation already exists")
		return invitation.PlatformInvitation{}, ErrInvitationAlreadyExists
	}
	if input.Role == platform.TenantRoleOwner {
		return invitation.PlatformInvitation{}, ErrOwnerRoleNotAllowed
	}
	code, err := generateSecureCode(invitationCodeLength)
	if err != nil {
		logger.Error().Err(err).Msg("failed to generate secure code")
		return invitation.PlatformInvitation{}, fault.ErrUnexpected
	}
	inv := invitation.PlatformInvitation{
		Email:            input.Email,
		PlatformTenantID: input.TenantID,
		Code:             code,
	}
	inv.GenerateID()

	createdEntity, err := s.invitationRepo.Create(ctx, inv)
	if err != nil {
		// A concurrent create can win the race after the FindByTenantIDAndEmail
		// check above passed, tripping the unique index. The repository reports
		// that as ErrInvitationExists, which is the same logical condition as the
		// pre-check, so it gets the same client error rather than a 500.
		if errors.Is(err, repository.ErrInvitationExists) {
			logger.Debug().
				Str("tenant_id", input.TenantID).
				Str("email", input.Email).
				Msg("invitation already exists (unique constraint)")
			return invitation.PlatformInvitation{}, ErrInvitationAlreadyExists
		}
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("email", input.Email).
			Err(err).
			Msg("failed to create invitation")
		return invitation.PlatformInvitation{}, fault.ErrUnexpected
	}

	logger.Info().
		Str("invitation_id", createdEntity.ID).
		Str("tenant_id", input.TenantID).
		Str("email", input.Email).
		Msg("invitation created")

	return createdEntity, nil
}

func (s *invitationService) DeleteInvitation(
	ctx context.Context, input invitation.DeleteInvitationInput,
) error {
	logger := s.logger.With().Str("operation", "DeleteInvitation").Logger()

	if err := validateStruct(input); err != nil {
		return err
	}
	err := s.invitationRepo.DeleteByTenantIDAndID(ctx, input.TenantID, input.InvitationID)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("invitation_id", input.InvitationID).
			Err(err).
			Msg("failed to delete invitation")
		return err
	}

	logger.Info().
		Str("tenant_id", input.TenantID).
		Str("invitation_id", input.InvitationID).
		Msg("invitation deleted")

	return nil
}

func (s *invitationService) SearchInvitation(
	ctx context.Context, input invitation.SearchInvitationInput,
) (search.Result[invitation.PlatformInvitation], error) {
	logger := s.logger.With().Str("operation", "SearchInvitation").Logger()

	if err := validateStruct(input); err != nil {
		return search.Result[invitation.PlatformInvitation]{}, err
	}
	result, err := s.invitationRepo.SearchByTenantID(ctx, input.TenantID, input.Request)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Err(err).
			Msg("failed to search invitations")
		return search.Result[invitation.PlatformInvitation]{}, err
	}
	return result, nil
}

func generateSecureCode(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
