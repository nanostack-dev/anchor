package service

import (
	"fmt"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

// ErrLicenseRevoked is returned when entitlements are read for a REVOKED
// license. 409: the request is valid but the license state forbids the read.
var ErrLicenseRevoked = fault.NewWithStatus(
	"LICENSE_REVOKED",
	"License is revoked; no entitlements can be resolved",
	http.StatusConflict,
)

// ErrLicenseExpired is returned when entitlements are read past the license's
// grace boundary (grace_until, or expires_at when no grace window is set).
var ErrLicenseExpired = fault.NewWithStatus(
	"LICENSE_EXPIRED",
	"License is expired beyond its grace period; no entitlements can be resolved",
	http.StatusConflict,
)

func NewLicenseNotFoundError(organizationID string) *fault.Error {
	return fault.NewWithStatus(
		"LICENSE_NOT_FOUND",
		fmt.Sprintf(
			"Organization %s has no license and the product has no default plan",
			organizationID,
		),
		http.StatusNotFound,
	)
}

func NewPlanNotFoundError(planID string) *fault.Error {
	return fault.NewWithStatus(
		"PLAN_NOT_FOUND",
		fmt.Sprintf("Plan %s does not exist in this product", planID),
		http.StatusNotFound,
	)
}

func NewPlanReferenceInvalidError(planID string) *fault.Error {
	return fault.BadRequest(
		"PLAN_REFERENCE_INVALID",
		fmt.Sprintf("Plan %s does not exist in this product", planID),
	).Metadata(map[string]any{"plan_id": planID})
}

func NewPlanKeyExistsError(key, productID string) *fault.Error {
	return fault.BadRequest(
		"PLAN_KEY_DUPLICATE", "A plan with this key already exists in the product",
	).Metadata(map[string]any{
		"plan_key":           key,
		errorDetailProductID: productID,
	})
}

func NewPlanInUseError(planID string, licenseCount int64) *fault.Error {
	return fault.BadRequest(
		"PLAN_IN_USE",
		fmt.Sprintf(
			"Plan %s cannot be deleted because %d license(s) still reference it",
			planID, licenseCount,
		),
	).Metadata(map[string]any{
		"plan_id":       planID,
		"license_count": licenseCount,
	})
}

func NewInvalidEntitlementsError(err error) *fault.Error {
	return fault.BadRequest("INVALID_ENTITLEMENTS", err.Error())
}

func NewInvalidLicenseStatusError(status string) *fault.Error {
	return fault.BadRequest(
		"INVALID_LICENSE_STATUS",
		fmt.Sprintf(
			"License status %q is invalid (allowed: ACTIVE, SUSPENDED, REVOKED)", status,
		),
	)
}

func NewInvalidLicenseGraceError() *fault.Error {
	return fault.BadRequest(
		"INVALID_LICENSE_GRACE",
		"grace_until must not be before expires_at",
	)
}
