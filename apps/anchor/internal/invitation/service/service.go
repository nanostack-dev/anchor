package service

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/organizationinvitation"
	"anchor/internal/events"
	"anchor/internal/invitation/repository"
	anchorrepository "anchor/internal/repository"
	anchorservice "anchor/internal/service"
)

// OrganizationInvitationService manages the invitations of an Organization.
// Anchor stores the invitation and its lifecycle and sends no email. A Product
// finds an invitation by the verified email address of the person who signed
// in, and accepts it by id.
type OrganizationInvitationService interface {
	Create(ctx context.Context, input organizationinvitation.CreateInput) (organizationinvitation.Invitation, error)
	Get(ctx context.Context, input organizationinvitation.GetInput) (organizationinvitation.Invitation, error)
	Search(
		ctx context.Context, input organizationinvitation.SearchInput,
	) (search.Result[organizationinvitation.Invitation], error)
	SearchInProduct(
		ctx context.Context, input organizationinvitation.SearchInProductInput,
	) (search.Result[organizationinvitation.Invitation], error)
	Update(ctx context.Context, input organizationinvitation.UpdateInput) (organizationinvitation.Invitation, error)
	Delete(ctx context.Context, input organizationinvitation.DeleteInput) error
	Accept(ctx context.Context, input organizationinvitation.AcceptInput) (organizationinvitation.Invitation, error)
}

type organizationInvitationService struct {
	invitationRepo    repository.Repository
	organizationRepo  anchorrepository.OrganizationRepository
	productRoleRepo   anchorrepository.ProductRoleRepository
	membershipService anchorservice.OrganizationMembershipService
	transactor        transactor.Transactor
	events            events.Emitter
	logger            zerolog.Logger
}

func NewOrganizationInvitationService(
	invitationRepo repository.Repository,
	organizationRepo anchorrepository.OrganizationRepository,
	productRoleRepo anchorrepository.ProductRoleRepository,
	membershipService anchorservice.OrganizationMembershipService,
	tx transactor.Transactor,
	eventEmitter events.Emitter,
	logger zerolog.Logger,
) OrganizationInvitationService {
	return &organizationInvitationService{
		invitationRepo:    invitationRepo,
		organizationRepo:  organizationRepo,
		productRoleRepo:   productRoleRepo,
		membershipService: membershipService,
		transactor:        tx,
		events:            eventEmitter,
		logger:            logger.With().Str("component", "organization_invitation_service").Logger(),
	}
}

func (s *organizationInvitationService) Create(
	ctx context.Context, input organizationinvitation.CreateInput,
) (organizationinvitation.Invitation, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return organizationinvitation.Invitation{}, err
	}

	now := time.Now()
	expiresAt := organizationinvitation.DefaultExpiryFrom(now)
	if input.ExpiresAt != nil {
		if !input.ExpiresAt.After(now) {
			return organizationinvitation.Invitation{}, errExpiryNotInFuture
		}
		expiresAt = *input.ExpiresAt
	}

	invitation := organizationinvitation.Invitation{
		ProductID:      input.ProductID,
		OrganizationID: input.OrganizationID,
		Email:          input.Email,
		RoleID:         input.RoleID,
		ExpiresAt:      expiresAt,
	}
	invitation.GenerateID()

	var created organizationinvitation.Invitation
	txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if lockErr := s.lockOrganization(txCtx, input.ProductID, input.OrganizationID); lockErr != nil {
			return lockErr
		}
		if roleErr := s.ensureRoleExists(txCtx, input.ProductID, input.RoleID); roleErr != nil {
			return roleErr
		}
		if pendingErr := s.ensureNoPendingInvitation(
			txCtx, input.ProductID, input.OrganizationID, input.Email,
		); pendingErr != nil {
			return pendingErr
		}
		if memberErr := s.ensureEmailIsNotMember(
			txCtx, input.ProductID, input.OrganizationID, input.Email,
		); memberErr != nil {
			return memberErr
		}
		var createErr error
		created, createErr = s.invitationRepo.Create(txCtx, invitation)
		if createErr != nil {
			return createErr
		}
		return s.emit(txCtx, events.OrganizationInvitationCreated, created)
	})
	if txErr != nil {
		return organizationinvitation.Invitation{}, txErr
	}

	return created, nil
}

func (s *organizationInvitationService) Get(
	ctx context.Context, input organizationinvitation.GetInput,
) (organizationinvitation.Invitation, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return organizationinvitation.Invitation{}, err
	}

	found, err := s.invitationRepo.FindByID(ctx, input.ProductID, input.OrganizationID, input.InvitationID)
	if err != nil {
		return organizationinvitation.Invitation{}, err
	}
	if found.IsAbsent() {
		return organizationinvitation.Invitation{}, errInvitationNotFound
	}

	return found.Value(), nil
}

func (s *organizationInvitationService) Search(
	ctx context.Context, input organizationinvitation.SearchInput,
) (search.Result[organizationinvitation.Invitation], error) {
	if err := validate.ValidateStruct(input); err != nil {
		return search.Result[organizationinvitation.Invitation]{}, err
	}

	foundOrganization, err := s.organizationRepo.FindByID(ctx, input.ProductID, input.OrganizationID)
	if err != nil {
		return search.Result[organizationinvitation.Invitation]{}, err
	}
	if foundOrganization.IsAbsent() {
		return search.Result[organizationinvitation.Invitation]{}, organization.ErrOrganizationNotFound
	}

	return s.invitationRepo.Search(ctx, input.ProductID, functional.Some(input.OrganizationID), input.Request)
}

func (s *organizationInvitationService) SearchInProduct(
	ctx context.Context, input organizationinvitation.SearchInProductInput,
) (search.Result[organizationinvitation.Invitation], error) {
	if err := validate.ValidateStruct(input); err != nil {
		return search.Result[organizationinvitation.Invitation]{}, err
	}

	return s.invitationRepo.Search(ctx, input.ProductID, functional.None[string](), input.Request)
}

func (s *organizationInvitationService) Update(
	ctx context.Context, input organizationinvitation.UpdateInput,
) (organizationinvitation.Invitation, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return organizationinvitation.Invitation{}, err
	}
	if !input.ExpiresAt.After(time.Now()) {
		return organizationinvitation.Invitation{}, errExpiryNotInFuture
	}

	var updated organizationinvitation.Invitation
	txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		current, findErr := s.lockAndFind(txCtx, input.ProductID, input.OrganizationID, input.InvitationID)
		if findErr != nil {
			return findErr
		}
		if statusErr := refuseUnlessPending(current); statusErr != nil {
			return statusErr
		}
		if roleErr := s.ensureRoleExists(txCtx, input.ProductID, input.RoleID); roleErr != nil {
			return roleErr
		}

		current.RoleID = input.RoleID
		current.ExpiresAt = input.ExpiresAt
		var updateErr error
		updated, updateErr = s.invitationRepo.UpdateRoleAndExpiry(txCtx, current)
		if updateErr != nil {
			return updateErr
		}
		return s.emit(txCtx, events.OrganizationInvitationUpdated, updated)
	})
	if txErr != nil {
		return organizationinvitation.Invitation{}, txErr
	}

	return updated, nil
}

func (s *organizationInvitationService) Delete(
	ctx context.Context, input organizationinvitation.DeleteInput,
) error {
	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		current, findErr := s.lockAndFind(txCtx, input.ProductID, input.OrganizationID, input.InvitationID)
		if findErr != nil {
			return findErr
		}
		if deleteErr := s.invitationRepo.Delete(
			txCtx, input.ProductID, input.OrganizationID, input.InvitationID,
		); deleteErr != nil {
			return deleteErr
		}
		return s.emit(txCtx, events.OrganizationInvitationDeleted, current)
	})
}

// Accept turns a pending invitation into a membership. The membership, the
// accepted mark and both events commit together or not at all. It never
// creates a Product User and never compares emails: the Product owns both.
func (s *organizationInvitationService) Accept(
	ctx context.Context, input organizationinvitation.AcceptInput,
) (organizationinvitation.Invitation, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return organizationinvitation.Invitation{}, err
	}

	var accepted organizationinvitation.Invitation
	txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		current, findErr := s.lockAndFind(txCtx, input.ProductID, input.OrganizationID, input.InvitationID)
		if findErr != nil {
			return findErr
		}
		if statusErr := refuseUnlessPending(current); statusErr != nil {
			return statusErr
		}

		if _, addErr := s.membershipService.AddMember(txCtx, organization.AddMemberInput{
			ProductID:      input.ProductID,
			OrganizationID: input.OrganizationID,
			ProductUserID:  input.ProductUserID,
			RoleID:         current.RoleID,
		}); addErr != nil {
			return addErr
		}

		var markErr error
		accepted, markErr = s.invitationRepo.MarkAccepted(
			txCtx, input.ProductID, input.OrganizationID, input.InvitationID, time.Now(),
		)
		if markErr != nil {
			return markErr
		}
		return s.events.Emit(txCtx, events.Event{
			Type:      events.OrganizationInvitationAccepted,
			ProductID: input.ProductID,
			Data: events.Data{
				events.FieldOrganizationID: input.OrganizationID,
				events.FieldInvitationID:   accepted.ID,
				events.FieldProductUserID:  input.ProductUserID,
			},
		})
	})
	if txErr != nil {
		return organizationinvitation.Invitation{}, txErr
	}

	return accepted, nil
}

func (s *organizationInvitationService) lockOrganization(
	ctx context.Context, productID, organizationID string,
) error {
	exists, err := s.invitationRepo.LockOrganization(ctx, productID, organizationID)
	if err != nil {
		return err
	}
	if !exists {
		return organization.ErrOrganizationNotFound
	}
	return nil
}

func (s *organizationInvitationService) lockAndFind(
	ctx context.Context, productID, organizationID, invitationID string,
) (organizationinvitation.Invitation, error) {
	if lockErr := s.lockOrganization(ctx, productID, organizationID); lockErr != nil {
		return organizationinvitation.Invitation{}, lockErr
	}
	found, err := s.invitationRepo.FindByID(ctx, productID, organizationID, invitationID)
	if err != nil {
		return organizationinvitation.Invitation{}, err
	}
	if found.IsAbsent() {
		return organizationinvitation.Invitation{}, errInvitationNotFound
	}
	return found.Value(), nil
}

func (s *organizationInvitationService) ensureRoleExists(
	ctx context.Context, productID, roleID string,
) error {
	found, err := s.productRoleRepo.FindByProductIDAndRoleID(ctx, productID, roleID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return anchorservice.NewBodyRoleNotFoundError(roleID)
	}
	return nil
}

// ensureNoPendingInvitation enforces the first integrity rule: at most one
// pending invitation per email per Organization. It runs under the
// Organization lock, so two concurrent writers cannot both pass it.
func (s *organizationInvitationService) ensureNoPendingInvitation(
	ctx context.Context, productID, organizationID, email string,
) error {
	pending, err := s.invitationRepo.FindPendingByEmail(ctx, productID, organizationID, email)
	if err != nil {
		return err
	}
	if pending.IsPresent() {
		return errPendingInvitationExists
	}
	return nil
}

// ensureEmailIsNotMember enforces the second integrity rule: no invitation for
// the email address of a member.
func (s *organizationInvitationService) ensureEmailIsNotMember(
	ctx context.Context, productID, organizationID, email string,
) error {
	isMember, err := s.invitationRepo.EmailBelongsToMember(ctx, productID, organizationID, email)
	if err != nil {
		return err
	}
	if isMember {
		return errEmailBelongsToMember
	}
	return nil
}

func refuseUnlessPending(invitation organizationinvitation.Invitation) error {
	switch invitation.StatusAt(time.Now()) {
	case organizationinvitation.StatusAccepted:
		return errInvitationAlreadyAccepted
	case organizationinvitation.StatusExpired:
		return errInvitationExpired
	case organizationinvitation.StatusPending:
		return nil
	}
	return nil
}

func (s *organizationInvitationService) emit(
	ctx context.Context, eventType events.Type, invitation organizationinvitation.Invitation,
) error {
	return s.events.Emit(ctx, events.Event{
		Type:      eventType,
		ProductID: invitation.ProductID,
		Data:      events.InvitationData(invitation.OrganizationID, invitation.ID),
	})
}
