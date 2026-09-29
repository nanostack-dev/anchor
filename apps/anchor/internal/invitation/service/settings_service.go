package service

import (
	"context"
	"strings"
	"time"

	"anchor/internal/domain/organizationinvitation"
	emailsvc "anchor/internal/email/service"
	"anchor/internal/invitation/repository"
	"anchor/internal/security"
)

// InvitationSettingsService reads and replaces the invitation settings of a
// Product. Changing them emits no Product event: it is an admin write.
type InvitationSettingsService interface {
	Get(ctx context.Context, input organizationinvitation.GetSettingsInput) (organizationinvitation.Settings, error)
	Update(
		ctx context.Context, input organizationinvitation.UpdateSettingsInput,
	) (organizationinvitation.Settings, error)
}

type invitationSettingsService struct {
	settingsRepo repository.SettingsRepository
	gate         anchorDeliveryGate
}

func NewInvitationSettingsService(
	settingsRepo repository.SettingsRepository,
	emailService emailsvc.EmailService,
) InvitationSettingsService {
	return &invitationSettingsService{
		settingsRepo: settingsRepo,
		gate:         anchorDeliveryGate{emailService: emailService},
	}
}

func (s *invitationSettingsService) Get(
	ctx context.Context, input organizationinvitation.GetSettingsInput,
) (organizationinvitation.Settings, error) {
	if err := validateStruct(input); err != nil {
		return organizationinvitation.Settings{}, err
	}
	return loadSettings(ctx, s.settingsRepo, input.ProductID)
}

func (s *invitationSettingsService) Update(
	ctx context.Context, input organizationinvitation.UpdateSettingsInput,
) (organizationinvitation.Settings, error) {
	if err := validateStruct(input); err != nil {
		return organizationinvitation.Settings{}, err
	}
	if input.AcceptURLTemplate != nil &&
		!strings.Contains(*input.AcceptURLTemplate, organizationinvitation.AcceptURLTokenPlaceholder) {
		return organizationinvitation.Settings{}, errAcceptURLTemplateWithoutToken
	}
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return organizationinvitation.Settings{}, err
	}

	settings := organizationinvitation.Settings{
		ProductID:          input.ProductID,
		InvitationDelivery: input.InvitationDelivery,
		EmailTemplateID:    input.EmailTemplateID,
		AcceptURLTemplate:  input.AcceptURLTemplate,
		DefaultExpiry:      time.Duration(input.DefaultExpirySeconds) * time.Second,
	}

	if templateErr := s.refuseUnknownTemplate(ctx, tenantID, settings); templateErr != nil {
		return organizationinvitation.Settings{}, templateErr
	}
	if settings.InvitationDelivery == organizationinvitation.DeliveryAnchor {
		unmet, gateErr := s.gate.unmetConditions(ctx, tenantID, settings)
		if gateErr != nil {
			return organizationinvitation.Settings{}, gateErr
		}
		if len(unmet) > 0 {
			return organizationinvitation.Settings{}, errAnchorDeliveryUnavailable(settingsUnavailableCode, unmet)
		}
	}

	return s.settingsRepo.Upsert(ctx, settings)
}

func (s *invitationSettingsService) refuseUnknownTemplate(
	ctx context.Context, tenantID string, settings organizationinvitation.Settings,
) error {
	if settings.EmailTemplateID == nil {
		return nil
	}
	exists, err := s.gate.templateExists(ctx, tenantID, settings)
	if err != nil {
		return err
	}
	if !exists {
		return errSettingsTemplateNotFound
	}
	return nil
}

func loadSettings(
	ctx context.Context, settingsRepo repository.SettingsRepository, productID string,
) (organizationinvitation.Settings, error) {
	found, err := settingsRepo.FindByProductID(ctx, productID)
	if err != nil {
		return organizationinvitation.Settings{}, err
	}
	return found.OrElse(organizationinvitation.DefaultSettings(productID)), nil
}
