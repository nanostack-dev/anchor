package license

import (
	"time"

	"anchor/internal/domain/plan"
)

type ListLicensesInput struct {
	ProductID string `json:"product_id" validate:"required,notblank"`
}

type GetLicenseInput struct {
	ProductID      string `json:"product_id"      validate:"required,notblank"`
	OrganizationID string `json:"organization_id" validate:"required,notblank"`
}

// PutLicenseInput assigns or fully replaces an organization's license.
type PutLicenseInput struct {
	ProductID              string            `json:"product_id"                         validate:"required,notblank"`
	OrganizationID         string            `json:"organization_id"                    validate:"required,notblank"`
	PlanID                 string            `json:"plan_id"                            validate:"required,notblank"`
	Status                 *Status           `json:"status,omitempty"`
	ExpiresAt              *time.Time        `json:"expires_at,omitempty"`
	GraceUntil             *time.Time        `json:"grace_until,omitempty"`
	EntitlementOverrides   plan.Entitlements `json:"entitlement_overrides,omitempty"`
	RefreshIntervalSeconds *int32            `json:"refresh_interval_seconds,omitempty" validate:"omitempty,min=60,max=2592000"`
}

type RevokeLicenseInput struct {
	ProductID      string `json:"product_id"      validate:"required,notblank"`
	OrganizationID string `json:"organization_id" validate:"required,notblank"`
}

type SuspendLicenseInput struct {
	ProductID      string `json:"product_id"      validate:"required,notblank"`
	OrganizationID string `json:"organization_id" validate:"required,notblank"`
}

type ReinstateLicenseInput struct {
	ProductID      string `json:"product_id"      validate:"required,notblank"`
	OrganizationID string `json:"organization_id" validate:"required,notblank"`
}

// GetEntitlementsInput resolves an organization's entitlement snapshot.
type GetEntitlementsInput struct {
	ProductID      string `json:"product_id"      validate:"required,notblank"`
	OrganizationID string `json:"organization_id" validate:"required,notblank"`
}
