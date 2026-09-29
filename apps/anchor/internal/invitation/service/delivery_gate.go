package service

import (
	"context"

	"anchor/internal/domain/email"
	"anchor/internal/domain/organizationinvitation"
	emailsvc "anchor/internal/email/service"
)

type anchorDeliveryGate struct {
	emailService emailsvc.EmailService
}

// unmetConditions lists every condition of Anchor delivery that is false for
// the settings. An empty list means Anchor can send the invitation email.
func (g anchorDeliveryGate) unmetConditions(
	ctx context.Context, tenantID string, settings organizationinvitation.Settings,
) ([]string, error) {
	mailerActive, err := g.emailService.IsMailerActive(ctx, tenantID, settings.ProductID)
	if err != nil {
		return nil, err
	}
	templateExists, err := g.templateExists(ctx, tenantID, settings)
	if err != nil {
		return nil, err
	}

	var unmet []string
	if !mailerActive {
		unmet = append(unmet, organizationinvitation.ConditionSMTPIntegrationActive)
	}
	if !templateExists {
		unmet = append(unmet, organizationinvitation.ConditionEmailTemplateSet)
	}
	if settings.AcceptURLTemplate == nil {
		unmet = append(unmet, organizationinvitation.ConditionAcceptURLTemplateSet)
	}
	return unmet, nil
}

func (g anchorDeliveryGate) templateExists(
	ctx context.Context, tenantID string, settings organizationinvitation.Settings,
) (bool, error) {
	if settings.EmailTemplateID == nil {
		return false, nil
	}
	found, err := g.emailService.GetTemplate(ctx, email.GetTemplateInput{
		TenantID:  tenantID,
		ProductID: settings.ProductID,
		ID:        *settings.EmailTemplateID,
	})
	if err != nil {
		return false, err
	}
	return found != nil, nil
}
