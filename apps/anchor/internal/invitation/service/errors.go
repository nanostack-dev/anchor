package service

import (
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"anchor/internal/domain/organizationinvitation"
)

var (
	errInvitationNotFound = fault.NotFound(
		"ORGANIZATION_INVITATION_NOT_FOUND",
		"This organization has no invitation with that identifier.",
	)

	errTokenNotFound = fault.BadRequest(
		"ORGANIZATION_INVITATION_TOKEN_NOT_FOUND",
		"This product has no invitation with that token.",
	)

	errPendingInvitationExists = fault.Conflict(
		"ORGANIZATION_INVITATION_ALREADY_PENDING",
		"A pending invitation already exists for this email address in this organization.",
	)

	errEmailBelongsToMember = fault.Conflict(
		"ORGANIZATION_INVITATION_EMAIL_IS_MEMBER",
		"This email address belongs to a member of this organization already.",
	)

	errInvitationAlreadyAccepted = fault.Conflict(
		"ORGANIZATION_INVITATION_ALREADY_ACCEPTED",
		"This invitation is accepted already.",
	)

	errInvitationExpired = fault.Conflict(
		"ORGANIZATION_INVITATION_EXPIRED",
		"This invitation is expired. Create a new invitation.",
	)

	errExpiryNotInFuture = fault.BadRequest(
		"ORGANIZATION_INVITATION_EXPIRY_NOT_IN_FUTURE",
		"The expiry of an invitation must be in the future.",
	)
)

var (
	errEmailNotSent = fault.Internal(
		"ORGANIZATION_INVITATION_EMAIL_SEND_FAILED",
		"Anchor could not send the invitation email, so the call had no effect. Try again later.",
	)

	errSettingsTemplateNotFound = fault.BadRequest(
		"ORGANIZATION_INVITATION_SETTINGS_EMAIL_TEMPLATE_NOT_FOUND",
		"This product has no email template with that identifier.",
	)

	errAcceptURLTemplateWithoutToken = fault.BadRequest(
		"ORGANIZATION_INVITATION_SETTINGS_ACCEPT_URL_TEMPLATE_WITHOUT_TOKEN",
		"The accept URL template must contain the "+organizationinvitation.AcceptURLTokenPlaceholder+" placeholder.",
	)
)

const (
	invitationUnavailableCode = "ORGANIZATION_INVITATION_ANCHOR_DELIVERY_UNAVAILABLE"
	settingsUnavailableCode   = "ORGANIZATION_INVITATION_SETTINGS_ANCHOR_DELIVERY_UNAVAILABLE"
)

func unmetConditionMessage(condition string) string {
	switch condition {
	case organizationinvitation.ConditionSMTPIntegrationActive:
		return "the SMTP integration of the product is not active"
	case organizationinvitation.ConditionEmailTemplateSet:
		return "no email template is chosen"
	case organizationinvitation.ConditionAcceptURLTemplateSet:
		return "no accept URL template is set"
	}
	return condition
}

func errAnchorDeliveryUnavailable(code string, unmetConditions []string) *fault.Error {
	reasons := make([]string, 0, len(unmetConditions))
	for _, condition := range unmetConditions {
		reasons = append(reasons, unmetConditionMessage(condition))
	}
	return fault.Conflict(
		code,
		"Anchor delivery is unavailable: "+strings.Join(reasons, ", and ")+".",
	).Metadata(map[string]any{"unmet_conditions": unmetConditions})
}
