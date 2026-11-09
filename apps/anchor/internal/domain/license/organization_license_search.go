package license

// SortFieldOrganizationLicense is the sort_by type of POST
// /licensing/organization-licenses/search. The generated search owns the
// values; this type gives the OpenAPI schema its Go type.
type SortFieldOrganizationLicense string

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
