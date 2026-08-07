package repository

import (
	"context"

	"anchor/internal/domain/license"
)

// SchemaRepository persists the per-Product license schema envelope. The
// declared fields live on SchemaFieldRepository so a schema edit can replace
// them wholesale without rewriting the envelope's identity.
//
// Every method is tenant-scoped. There is no *Internal variant: nothing in the
// licensing write path runs without an authenticated tenant.
type SchemaRepository interface {
	// FindByProduct returns the Product's schema, or nil when it has never
	// declared one. Fields are not populated; use SchemaFieldRepository.
	FindByProduct(
		ctx context.Context, tenantID string, productID string,
	) (*license.Schema, error)
	Create(
		ctx context.Context, schema license.Schema,
	) (license.Schema, error)
	Update(
		ctx context.Context, tenantID string, schema license.Schema,
	) (license.Schema, error)
	DeleteByProduct(
		ctx context.Context, tenantID string, productID string,
	) error
}

// SchemaFieldRepository persists the license fields declared by a schema.
type SchemaFieldRepository interface {
	// ListBySchema returns the schema's fields in declaration order.
	ListBySchema(
		ctx context.Context, schemaID string,
	) ([]license.Field, error)
	// ReplaceAll swaps the schema's entire field set for the one supplied and
	// returns what was written. A schema is one declaration, so the replacement
	// is the only edit shape that has an unambiguous meaning for removals.
	// Callers run it inside a transaction so a reader never observes the gap
	// between the delete and the insert.
	ReplaceAll(
		ctx context.Context, schemaID string, fields []license.Field,
	) ([]license.Field, error)
}

// TemplateRepository persists a Product's license templates.
//
// A template's values live on the row itself rather than in a child table, so
// there is no second repository here: a template is written and read whole.
//
// Every method is tenant-scoped and product-scoped. There is no *Internal
// variant: nothing in the licensing write path runs without an authenticated
// tenant.
type TemplateRepository interface {
	// FindByID returns the template, or nil when the Product has none with that
	// identifier. Scoping by product as well as by identifier is what stops a
	// caller reading another Product's template by guessing its KSUID.
	FindByID(
		ctx context.Context, tenantID string, productID string, templateID string,
	) (*license.Template, error)
	// FindByName returns the Product's template with that name, or nil. Names are
	// unique within a Product, so this is how a conflicting create is detected
	// before the unique index has to decide it.
	FindByName(
		ctx context.Context, tenantID string, productID string, name string,
	) (*license.Template, error)
	// ListByProduct returns the Product's templates ordered by name.
	ListByProduct(
		ctx context.Context, tenantID string, productID string,
	) ([]license.Template, error)
	Create(
		ctx context.Context, template license.Template,
	) (license.Template, error)
	Update(
		ctx context.Context, tenantID string, template license.Template,
	) (license.Template, error)
	DeleteByID(
		ctx context.Context, tenantID string, productID string, templateID string,
	) error
}

// OrganizationLicenseRepository persists one Organization's copy of a template's
// values.
//
// There is no list method and no lookup by identifier: a license is a singleton
// on its Organization, so the Organization is the only address it has.
//
// Every method is tenant-scoped and product-scoped. There is no *Internal
// variant: nothing in the licensing write path runs without an authenticated
// tenant.
type OrganizationLicenseRepository interface {
	// FindByOrganization returns the Organization's license, or nil when it has
	// never been instantiated. Scoping by product as well is what stops a caller
	// reading another Product's license by guessing a KSUID.
	FindByOrganization(
		ctx context.Context, tenantID string, productID string, organizationID string,
	) (*license.OrganizationLicense, error)
	Create(
		ctx context.Context, organizationLicense license.OrganizationLicense,
	) (license.OrganizationLicense, error)
	Update(
		ctx context.Context, tenantID string, organizationLicense license.OrganizationLicense,
	) (license.OrganizationLicense, error)
}
