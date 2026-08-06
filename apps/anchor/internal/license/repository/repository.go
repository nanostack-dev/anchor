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
