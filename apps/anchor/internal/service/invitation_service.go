package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/domain/invitation"
	"anchor/internal/domain/platform"
	"anchor/internal/mapper"
	"anchor/internal/repository"
	"anchor/internal/searchx"
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
	logger           *slog.Logger
	mapper           *mapper.InvitationMapper
}

func NewInvitationService(
	platformUserRepo repository.PlatformTenantUserRepository,
	invitationRepo repository.InvitationRepository, logger *slog.Logger,
) InvitationService {
	return &invitationService{
		invitationRepo:   invitationRepo,
		logger:           logger.With(slog.String("component", "invitation_service")),
		mapper:           mapper.NewInvitationMapper(),
		platformUserRepo: platformUserRepo,
	}
}

func (s *invitationService) CreateInvitation(
	ctx context.Context, input invitation.CreateInvitationInput,
) (invitation.PlatformInvitation, error) {
	logger := s.logger.With(slog.String("operation", "CreateInvitation"))

	if err := validate.ValidateStruct(input); err != nil {
		return invitation.PlatformInvitation{}, err
	}
	foundPlatformUser, err := s.platformUserRepo.FindByTenantIDAndEmail(
		ctx, input.TenantID, input.Email,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to find user by email",
			slog.String("tenant_id", input.TenantID),
			slog.String("email", input.Email),
			log.Err(err),
		)
		return invitation.PlatformInvitation{}, fault.ErrUnexpected
	}
	if foundPlatformUser.IsPresent() {
		logger.DebugContext(ctx, "user already exists",
			slog.String("tenant_id", input.TenantID),
			slog.String("email", input.Email),
		)
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
		logger.ErrorContext(ctx, "failed to find invitation by email",
			slog.String("tenant_id", input.TenantID),
			slog.String("email", input.Email),
			log.Err(err),
		)
		return invitation.PlatformInvitation{}, fault.ErrUnexpected
	}
	if foundInvitation.IsPresent() {
		logger.DebugContext(ctx, "invitation already exists",
			slog.String("tenant_id", input.TenantID),
			slog.String("email", input.Email),
		)
		return invitation.PlatformInvitation{}, ErrInvitationAlreadyExists
	}
	if input.Role == platform.TenantRoleOwner {
		return invitation.PlatformInvitation{}, ErrOwnerRoleNotAllowed
	}
	code, err := generateSecureCode(invitationCodeLength)
	if err != nil {
		logger.ErrorContext(ctx, "failed to generate secure code", log.Err(err))
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
			logger.DebugContext(ctx, "invitation already exists (unique constraint)",
				slog.String("tenant_id", input.TenantID),
				slog.String("email", input.Email),
			)
			return invitation.PlatformInvitation{}, ErrInvitationAlreadyExists
		}
		logger.ErrorContext(ctx, "failed to create invitation",
			slog.String("tenant_id", input.TenantID),
			slog.String("email", input.Email),
			log.Err(err),
		)
		return invitation.PlatformInvitation{}, fault.ErrUnexpected
	}

	logger.InfoContext(ctx, "invitation created",
		slog.String("invitation_id", createdEntity.ID),
		slog.String("tenant_id", input.TenantID),
		slog.String("email", input.Email),
	)

	return createdEntity, nil
}

func (s *invitationService) DeleteInvitation(
	ctx context.Context, input invitation.DeleteInvitationInput,
) error {
	logger := s.logger.With(slog.String("operation", "DeleteInvitation"))

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}
	err := s.invitationRepo.DeleteByTenantIDAndID(ctx, input.TenantID, input.InvitationID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to delete invitation",
			slog.String("tenant_id", input.TenantID),
			slog.String("invitation_id", input.InvitationID),
			log.Err(err),
		)
		return err
	}

	logger.InfoContext(ctx, "invitation deleted",
		slog.String("tenant_id", input.TenantID),
		slog.String("invitation_id", input.InvitationID),
	)

	return nil
}

func (s *invitationService) SearchInvitation(
	ctx context.Context, input invitation.SearchInvitationInput,
) (search.Result[invitation.PlatformInvitation], error) {
	logger := s.logger.With(slog.String("operation", "SearchInvitation"))

	if err := input.Validate(); err != nil {
		return search.Result[invitation.PlatformInvitation]{}, err
	}
	result, err := s.invitationRepo.SearchByTenantID(ctx, input.TenantID, input.Request)
	if err != nil {
		if !searchx.IsClientError(err) {
			logger.ErrorContext(ctx, "failed to search invitations",
				slog.String("tenant_id", input.TenantID),
				log.Err(err),
			)
		}
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
