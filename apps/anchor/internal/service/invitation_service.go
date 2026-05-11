package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

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

	if err := toolkit.ValidateStruct(input); err != nil {
		return invitation.PlatformInvitation{}, err
	}
	optPlatformUser, err := s.platformUserRepo.FindByTenantIDAndEmail(
		ctx, input.TenantID, input.Email,
		nil,
	)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("email", input.Email).
			Err(err).
			Msg("failed to find user by email")
		return invitation.PlatformInvitation{}, toolkit.ErrUnexpected
	}
	if optPlatformUser != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("email", input.Email).
			Msg("user already exists")
		return invitation.PlatformInvitation{}, toolkit.NewNanostackErrorsWithStatus(
			"INVITATION_USER_ALREADY_EXISTS",
			"This email address is already associated with an existing user account. "+
				"Please check if they are already a member, or try inviting a different email.",
			http.StatusBadRequest,
		)
	}
	optInvitation, err := s.invitationRepo.FindByTenantIDAndEmail(
		ctx, input.TenantID, input.Email, nil,
	)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("email", input.Email).
			Err(err).
			Msg("failed to find invitation by email")
		return invitation.PlatformInvitation{}, toolkit.ErrUnexpected
	}
	if optInvitation != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("email", input.Email).
			Msg("invitation already exists")
		return invitation.PlatformInvitation{}, toolkit.NewNanostackErrorsWithStatus(
			"INVITATION_ALREADY_EXISTS",
			"This email address is already associated with an existing invitation. "+
				"Please check if they are already a member, or try inviting a different email.",
			http.StatusBadRequest,
		)
	}
	if input.Role == platform.TenantRoleOwner {
		return invitation.PlatformInvitation{}, ErrOwnerRoleNotAllowed
	}
	code, err := generateSecureCode(invitationCodeLength)
	if err != nil {
		logger.Error().Err(err).Msg("failed to generate secure code")
		return invitation.PlatformInvitation{}, toolkit.ErrUnexpected
	}
	inv := invitation.PlatformInvitation{
		Email:            input.Email,
		PlatformTenantID: input.TenantID,
		Code:             code,
	}
	inv.GenerateID()

	createdEntity, err := s.invitationRepo.Create(ctx, inv, nil)
	if err != nil {
		logger.Error().
			Str("tenant_id", input.TenantID).
			Str("email", input.Email).
			Err(err).
			Msg("failed to create invitation")
		return invitation.PlatformInvitation{}, toolkit.ErrUnexpected
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

	if err := toolkit.ValidateStruct(input); err != nil {
		return err
	}
	err := s.invitationRepo.DeleteByTenantIDAndID(ctx, input.TenantID, input.InvitationID, nil)
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

	if err := toolkit.ValidateStruct(input); err != nil {
		return search.Result[invitation.PlatformInvitation]{}, err
	}
	result, err := s.invitationRepo.SearchByTenantID(ctx, input.TenantID, input.Request, nil)
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
