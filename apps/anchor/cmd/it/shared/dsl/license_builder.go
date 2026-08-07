package itdsl

import (
	"context"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// Arrange-only builders for the licensing subsystem. They put a product into
// the state a test starts from. The act under test goes through the handles in
// the test's own package, never through here.

// LicenseTemplate is a DSL-level license template runtime context.
type LicenseTemplate struct {
	ID           string
	ProductAlias string
	Name         string
	Values       nanostackClient.LicenseTemplateValues
}

// OrganizationLicense is a DSL-level organization license runtime context.
type OrganizationLicense struct {
	ID             string
	ProductAlias   string
	OrganizationID string
	TemplateID     string
	Values         nanostackClient.LicenseTemplateValues
}

// LicenseTemplateOpts configures a license template under a product alias.
type LicenseTemplateOpts struct {
	Alias        string
	ProductAlias string
	Name         string
	Description  *string
	Values       nanostackClient.LicenseTemplateValues
}

// OrganizationLicenseOpts instantiates a template onto an organization. Both
// are named by alias, so a test states the relationship rather than threading
// identifiers through its setup.
type OrganizationLicenseOpts struct {
	Alias             string
	ProductAlias      string
	OrganizationAlias string
	TemplateAlias     string
}

// LicenseTemplate defines a license template under a product alias. The product
// must already declare a license schema the values satisfy.
func (b *Builder) LicenseTemplate(opts LicenseTemplateOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "license template alias is required")
	require.NotEmpty(b.t, opts.ProductAlias, "product alias is required")
	require.NotEmpty(b.t, opts.Name, "license template name is required")
	_, exists := b.licenseTemplates[opts.Alias]
	require.False(b.t, exists, "license template alias '%s' already exists", opts.Alias)

	productCtx, ok := b.products[opts.ProductAlias]
	require.True(b.t, ok, "unknown product alias '%s'", opts.ProductAlias)

	values := opts.Values
	if values == nil {
		values = nanostackClient.LicenseTemplateValues{}
	}

	apiKeyClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"license_template:create"})
	resp, err := apiKeyClient.CreateLicenseTemplateWithResponse(
		context.Background(),
		productCtx.ProductID,
		nanostackClient.CreateLicenseTemplateJSONRequestBody{
			Name:        opts.Name,
			Description: opts.Description,
			Values:      values,
		},
	)
	require.NoError(b.t, err)
	require.NotNil(b.t, resp.JSON201, "license template creation failed: %s", string(resp.Body))

	b.licenseTemplates[opts.Alias] = &LicenseTemplate{
		ID:           resp.JSON201.Id,
		ProductAlias: opts.ProductAlias,
		Name:         resp.JSON201.Name,
		Values:       resp.JSON201.Values,
	}
	return b
}

// OrganizationLicense instantiates a template onto an organization, giving that
// organization its own copy of the template's values.
func (b *Builder) OrganizationLicense(opts OrganizationLicenseOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "organization license alias is required")
	require.NotEmpty(b.t, opts.ProductAlias, "product alias is required")
	require.NotEmpty(b.t, opts.OrganizationAlias, "organization alias is required")
	require.NotEmpty(b.t, opts.TemplateAlias, "license template alias is required")
	_, exists := b.organizationLicenses[opts.Alias]
	require.False(b.t, exists, "organization license alias '%s' already exists", opts.Alias)

	productCtx, ok := b.products[opts.ProductAlias]
	require.True(b.t, ok, "unknown product alias '%s'", opts.ProductAlias)
	organization, orgExists := b.organizations[opts.OrganizationAlias]
	require.True(b.t, orgExists, "unknown organization alias '%s'", opts.OrganizationAlias)
	template, templateExists := b.licenseTemplates[opts.TemplateAlias]
	require.True(b.t, templateExists, "unknown license template alias '%s'", opts.TemplateAlias)

	apiKeyClient, _ := productCtx.CreateAPIKeyClientWithScopes(
		[]string{"organization_license:create"},
	)
	resp, err := apiKeyClient.InstantiateOrganizationLicenseWithResponse(
		context.Background(),
		productCtx.ProductID,
		organization.ID,
		nanostackClient.InstantiateOrganizationLicenseJSONRequestBody{TemplateId: template.ID},
	)
	require.NoError(b.t, err)
	require.NotNil(b.t, resp.JSON201, "license instantiation failed: %s", string(resp.Body))

	b.organizationLicenses[opts.Alias] = &OrganizationLicense{
		ID:             resp.JSON201.Id,
		ProductAlias:   opts.ProductAlias,
		OrganizationID: resp.JSON201.OrganizationId,
		TemplateID:     resp.JSON201.TemplateId,
		Values:         resp.JSON201.Values,
	}
	return b
}

// LicenseTemplate returns the license template context for an alias.
func (s *State) LicenseTemplate(alias string) *LicenseTemplate {
	s.t.Helper()
	template, ok := s.licenseTemplates[alias]
	require.True(s.t, ok, "unknown license template alias '%s'", alias)
	return template
}

// OrganizationLicense returns the organization license context for an alias.
func (s *State) OrganizationLicense(alias string) *OrganizationLicense {
	s.t.Helper()
	organizationLicense, ok := s.organizationLicenses[alias]
	require.True(s.t, ok, "unknown organization license alias '%s'", alias)
	return organizationLicense
}
