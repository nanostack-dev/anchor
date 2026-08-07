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
	// FindByName returns the Product's active template with that name, or nil.
	// Names are unique among a Product's active templates, so this is how a
	// conflicting create is detected before the unique index has to decide it.
	// Archived templates are skipped, because archiving frees the name.
	FindByName(
		ctx context.Context, tenantID string, productID string, name string,
	) (*license.Template, error)
	// ListByProduct returns the Product's templates ordered by name, every status
	// unless one is named.
	ListByProduct(
		ctx context.Context, tenantID string, productID string, status *license.TemplateStatus,
	) ([]license.Template, error)
	Create(
		ctx context.Context, template license.Template,
	) (license.Template, error)
	// Update rewrites a template's name, description and values. Its status is
	// excluded — withdrawing a tier goes through Archive, so no edit path can
	// change it by accident.
	Update(
		ctx context.Context, tenantID string, template license.Template,
	) (license.Template, error)
	// Archive marks the template withdrawn and returns it. Prefer this once a
	// template might have customers: an Organization's license names the
	// template it came from, and that has to keep resolving.
	Archive(
		ctx context.Context, tenantID string, productID string, templateID string,
	) (license.Template, error)
	// Delete removes the row outright. The foreign key added by migration 000028
	// refuses this while any Organization license names the template, so callers
	// check with OrganizationLicenseRepository.CountLicensesForTemplate first;
	// this exists for the template nobody was ever licensed from. See
	// docs/adr/0011-unreferenced-license-template-can-be-deleted.md.
	Delete(
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
	// CountLicensesForTemplate reports how many Organization licenses in this
	// Product still name the given template. A template delete checks this
	// before the write, mirroring how CountMembershipAssignments guards a
	// product role delete.
	CountLicensesForTemplate(
		ctx context.Context, tenantID string, productID string, templateID string,
	) (int, error)
}

// UsageObservationRepository persists what an Organization has used.
//
// Append is the only write there is. An observation is never updated and never
// deleted, so a correction is a later observation rather than an edit of an
// earlier one — that is what makes the table a history instead of a current
// value with a timestamp on it. Reading the series back arrives with the usage
// history endpoint.
//
// Every method is tenant-scoped and product-scoped. There is no *Internal
// variant: nothing in the licensing write path runs without an authenticated
// tenant.
type UsageObservationRepository interface {
	Append(
		ctx context.Context, observation license.UsageObservation,
	) (license.UsageObservation, error)
}
