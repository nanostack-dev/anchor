package license

import "github.com/nanostack-dev/nanostack-framework/pkg/search"

// SearchOrganizationLicenseFilter narrows a search over a Product's
// Organizations and the licenses they hold.
type SearchOrganizationLicenseFilter struct {
	OrganizationIDs    []string
	LicenseTemplateIDs []string
	// Licensed keeps only Organizations that hold a license when true, only
	// those that hold none when false, and both when nil.
	Licensed *bool
}

// SortFieldOrganizationLicense names a column a search may order by.
type SortFieldOrganizationLicense string

const (
	SortFieldOrganizationLicenseOrganizationName SortFieldOrganizationLicense = "organization_name"
	SortFieldOrganizationLicenseInstantiatedAt   SortFieldOrganizationLicense = "instantiated_at"
)

// SearchOrganizationLicensesInput reads a page of the Product's customer book:
// each Organization and the license it holds, if any.
type SearchOrganizationLicensesInput struct {
	TenantID  string `validate:"required,notblank"`
	ProductID string `validate:"required,notblank"`
	Request   search.Request[SearchOrganizationLicenseFilter, SortFieldOrganizationLicense]
}

// OrganizationLicenseSummary is one Organization and the license it holds.
//
// The Organization is the subject, so one that has never been licensed is a
// result with a nil License rather than an absent row: "who is on no tier at
// all" is half of what this search is asked.
type OrganizationLicenseSummary struct {
	OrganizationID   string
	OrganizationName string
	License          *OrganizationLicense
}
