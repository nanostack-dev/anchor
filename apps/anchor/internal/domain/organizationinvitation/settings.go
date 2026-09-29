package organizationinvitation

import "time"

type Delivery string

const (
	DeliveryAnchor  Delivery = "anchor"
	DeliveryProduct Delivery = "product"
)

const (
	AcceptURLTokenPlaceholder = "{token}"
	DefaultExpiry             = 7 * 24 * time.Hour
)

const (
	ConditionSMTPIntegrationActive = "smtp_integration_active"
	ConditionEmailTemplateSet      = "email_template_set"
	ConditionAcceptURLTemplateSet  = "accept_url_template_set"
)

const (
	TemplateVariableAcceptURL        = "accept_url"
	TemplateVariableOrganizationName = "organization_name"
	TemplateVariableRoleName         = "role_name"
	TemplateVariableInviteeEmail     = "invitee_email"
	TemplateVariableExpiresAt        = "expires_at"
)

// Settings is the invitation configuration of one Product. A Product that never
// changed it reads DefaultSettings.
type Settings struct {
	ProductID          string
	InvitationDelivery Delivery
	EmailTemplateID    *string
	AcceptURLTemplate  *string
	DefaultExpiry      time.Duration
}

func DefaultSettings(productID string) Settings {
	return Settings{
		ProductID:          productID,
		InvitationDelivery: DeliveryProduct,
		DefaultExpiry:      DefaultExpiry,
	}
}

func (s Settings) ExpiryFrom(now time.Time) time.Time {
	return now.Add(s.DefaultExpiry)
}

type GetSettingsInput struct {
	ProductID string `validate:"required,notblank"`
}

type UpdateSettingsInput struct {
	ProductID            string   `validate:"required,notblank"`
	InvitationDelivery   Delivery `validate:"required,oneof=anchor product"`
	EmailTemplateID      *string  `validate:"omitempty,notblank"`
	AcceptURLTemplate    *string  `validate:"omitempty,notblank,max=2048"`
	DefaultExpirySeconds int64    `validate:"required,min=3600,max=7776000"`
}
