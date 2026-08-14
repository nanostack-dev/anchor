package itdsl

import (
	"context"

	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

// LicenseSchema is a DSL-level license schema runtime context.
type LicenseSchema struct {
	ID           string
	ProductAlias string
	Fields       []nanostackClient.LicenseFieldResponse
}

// FieldByName returns the declared field with the given name, or nil.
func (s *LicenseSchema) FieldByName(name string) *nanostackClient.LicenseFieldResponse {
	for i := range s.Fields {
		if s.Fields[i].Name == name {
			return &s.Fields[i]
		}
	}
	return nil
}

// LicenseSchemaOpts configures license schema creation linked to a product alias.
//
// A product declares exactly one license schema, so the alias identifies the
// schema in test state rather than distinguishing several per product.
type LicenseSchemaOpts struct {
	Alias        string
	ProductAlias string
	Description  *string
	Fields       []nanostackClient.LicenseFieldDeclaration
}

// LicenseField builds one field declaration. rules may be nil for a field the
// test does not constrain. usageShape is required for a LIMIT field and must
// be nil for every other type — pass it as new(nanostackClient.GAUGE) or
// new(nanostackClient.WINDOWEDCOUNTER).
//
// There is no required flag to pass: every license template must set every
// field its schema declares.
func LicenseField(
	name string,
	fieldType nanostackClient.LicenseFieldType,
	fieldRules *nanostackClient.LicenseFieldRules,
	usageShape *nanostackClient.UsageShape,
) nanostackClient.LicenseFieldDeclaration {
	return nanostackClient.LicenseFieldDeclaration{
		Name:       name,
		Type:       fieldType,
		Rules:      fieldRules,
		UsageShape: usageShape,
	}
}

// LicenseSchema declares a license schema under a product alias.
func (b *Builder) LicenseSchema(opts LicenseSchemaOpts) *Builder {
	b.t.Helper()
	require.NotEmpty(b.t, opts.Alias, "license schema alias is required")
	require.NotEmpty(b.t, opts.ProductAlias, "product alias is required")
	_, exists := b.licenseSchemas[opts.Alias]
	require.False(b.t, exists, "license schema alias '%s' already exists", opts.Alias)

	productCtx, ok := b.products[opts.ProductAlias]
	require.True(b.t, ok, "unknown product alias '%s'", opts.ProductAlias)

	fields := opts.Fields
	if fields == nil {
		fields = []nanostackClient.LicenseFieldDeclaration{}
	}

	apiKeyClient, _ := productCtx.CreateAPIKeyClientWithScopes([]string{"license_schema:create"})
	resp, err := apiKeyClient.CreateLicenseSchemaWithResponse(
		context.Background(),
		productCtx.ProductID,
		nanostackClient.CreateLicenseSchemaJSONRequestBody{
			Description: opts.Description,
			Fields:      fields,
		},
	)
	require.NoError(b.t, err)
	require.NotNil(b.t, resp.JSON201, "license schema creation failed: %s", string(resp.Body))

	b.licenseSchemas[opts.Alias] = &LicenseSchema{
		ID:           resp.JSON201.Id,
		ProductAlias: opts.ProductAlias,
		Fields:       resp.JSON201.Fields,
	}
	return b
}

// LicenseSchema returns the license schema context for an alias.
func (s *State) LicenseSchema(alias string) *LicenseSchema {
	s.t.Helper()
	schema, ok := s.licenseSchemas[alias]
	require.True(s.t, ok, "unknown license schema alias '%s'", alias)
	return schema
}
