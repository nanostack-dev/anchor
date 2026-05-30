package repository

import (
	"context"
	"time"

	"anchor/internal/domain/email"

	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
)

// TemplateRepository persists per-product email template envelopes (identity
// + draft/published version pointers). Authoring content lives on
// TemplateVersionRepository so PUBLISHED rows are immutable.
//
// Concrete implementation lands once `dbgen` regenerates jet models against
// migration 000014. The interface lives here so the email service can be
// wired and unit-tested with mocks before the impl exists.
type TemplateRepository interface {
	FindByID(
		ctx context.Context, tenantID string, productID string, id string, options *jetx.DBOptions,
	) (*email.Template, error)
	FindBySlug(
		ctx context.Context, tenantID string, productID string, slug string, options *jetx.DBOptions,
	) (*email.Template, error)
	// FindBySlugInternal is reserved for trusted system-internal paths that
	// dispatch transactional emails via product API key (no authenticated
	// tenant context). Must NOT be called from tenant-facing API handlers.
	FindBySlugInternal(
		ctx context.Context, productID string, slug string, options *jetx.DBOptions,
	) (*email.Template, error)
	List(
		ctx context.Context, tenantID string, productID string, limit int64, offset int64, options *jetx.DBOptions,
	) ([]email.Template, error)
	Create(
		ctx context.Context, template email.Template, options *jetx.DBOptions,
	) (email.Template, error)
	Update(
		ctx context.Context, tenantID string, template email.Template, options *jetx.DBOptions,
	) (email.Template, error)
	// SaveExamples replaces the example_data JSONB column for the given
	// template in a single targeted UPDATE (no full read-then-write).
	SaveExamples(
		ctx context.Context, tenantID string, productID string, templateID string, examples []email.TemplateExample, options *jetx.DBOptions,
	) error
	// SetVersionPointers atomically updates the draft/published version
	// pointers on the template envelope. Used by Publish to swap a draft
	// into the published slot in a single statement.
	SetVersionPointers(
		ctx context.Context, tenantID string, templateID string, draftVersionID *string, publishedVersionID *string, options *jetx.DBOptions,
	) error
	DeleteByID(
		ctx context.Context, tenantID string, productID string, id string, options *jetx.DBOptions,
	) error
}

// TemplateVersionRepository persists the snapshots of authoring content
// (subject, bodies, variables) attached to a Template. Each Template owns
// at most one DRAFT row and one PUBLISHED row at any time; older PUBLISHED
// rows are moved to ARCHIVED on republish.
type TemplateVersionRepository interface {
	FindByID(
		ctx context.Context, id string, options *jetx.DBOptions,
	) (*email.TemplateVersion, error)
	// FindCurrentDraft returns the template's current DRAFT version, or nil
	// when no draft exists (e.g. immediately after a publish before a new
	// draft has been opened).
	FindCurrentDraft(
		ctx context.Context, templateID string, options *jetx.DBOptions,
	) (*email.TemplateVersion, error)
	// FindCurrentPublished returns the template's currently-published
	// version, or nil when the template has never been published.
	FindCurrentPublished(
		ctx context.Context, templateID string, options *jetx.DBOptions,
	) (*email.TemplateVersion, error)
	List(
		ctx context.Context, templateID string, limit int64, offset int64, options *jetx.DBOptions,
	) ([]email.TemplateVersion, error)
	Create(
		ctx context.Context, version email.TemplateVersion, options *jetx.DBOptions,
	) (email.TemplateVersion, error)
	Update(
		ctx context.Context, version email.TemplateVersion, options *jetx.DBOptions,
	) (email.TemplateVersion, error)
	// UpdateStatus transitions a version between DRAFT/PUBLISHED/ARCHIVED.
	// Used by Publish to atomically move the previous published row to
	// ARCHIVED and the draft row to PUBLISHED inside one transaction.
	UpdateStatus(
		ctx context.Context, id string, status email.TemplateVersionStatus, publishedAt *time.Time, options *jetx.DBOptions,
	) error
	// NextVersionNumber returns the next monotonic version number to assign
	// to a new version row for a template.
	NextVersionNumber(
		ctx context.Context, templateID string, options *jetx.DBOptions,
	) (int32, error)
}

// SendRecordRepository persists every send attempt for audit + idempotency.
type SendRecordRepository interface {
	// CreateInternal inserts the record without tenant scoping; used by the
	// dispatcher which already validated tenancy upstream. The "Internal"
	// suffix matches the project convention for internal-only repo methods.
	CreateInternal(
		ctx context.Context, record email.SendRecord, options *jetx.DBOptions,
	) (email.SendRecord, error)
	// Create inserts the record under the given tenant and product.
	Create(
		ctx context.Context, tenantID string, productID string, record email.SendRecord, options *jetx.DBOptions,
	) (email.SendRecord, error)
	// UpdateStatusInternal updates the send record status by ID without tenant
	// scoping; used by async dispatchers where no tenant context exists.
	UpdateStatusInternal(
		ctx context.Context, id string, status email.SendStatus, lastError *string, sentAt *time.Time, options *jetx.DBOptions,
	) error
	// UpdateStatus updates the status of a send record, scoped to the tenant.
	UpdateStatus(
		ctx context.Context, tenantID string, id string, status email.SendStatus, lastError *string, sentAt *time.Time, options *jetx.DBOptions,
	) error
	// FindByDedupeKeyInternal looks up a send by dedupe key without tenant
	// scoping; used by async dispatchers.
	FindByDedupeKeyInternal(
		ctx context.Context, productID string, dedupeKey string, options *jetx.DBOptions,
	) (*email.SendRecord, error)
	// FindByDedupeKey returns a send record matching the dedupe key within the
	// specified tenant and product.
	FindByDedupeKey(
		ctx context.Context, tenantID string, productID string, dedupeKey string, options *jetx.DBOptions,
	) (*email.SendRecord, error)
	FindByID(
		ctx context.Context, tenantID string, productID string, id string, options *jetx.DBOptions,
	) (*email.SendRecord, error)
	List(
		ctx context.Context, input email.ListSendsInput, options *jetx.DBOptions,
	) ([]email.SendRecord, error)
	// CountSinceInternal returns the number of sends for a product since the
	// given timestamp. Used by the rate limiter; bypasses tenant scoping
	// because the rate-limit decision is keyed on (product_id, since).
	CountSinceInternal(
		ctx context.Context, productID string, since time.Time, options *jetx.DBOptions,
	) (int64, error)
	// CountSince returns the number of sends for a product and tenant since
	// the given timestamp.
	CountSince(
		ctx context.Context, tenantID string, productID string, since time.Time, options *jetx.DBOptions,
	) (int64, error)
}
