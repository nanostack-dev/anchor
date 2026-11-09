package repository

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/email"
)

// TemplateRepository persists per-product email template envelopes (identity
// + draft/published version pointers). Authoring content lives on
// TemplateVersionRepository so PUBLISHED rows are immutable.
//
// The interface lives in this package so the email service can be wired
// and unit-tested with mocks; the Postgres implementation is next to it.
// Methods without a hand-written body are derived from their name by
// go tool nano gen (template_repository_repo.gen.go).
type TemplateRepository interface {
	FindByPlatformTenantIDAndProductIDAndID(
		ctx context.Context, platformTenantID string, productID string, id string,
	) (functional.Option[email.Template], error)
	FindByPlatformTenantIDAndProductIDAndSlug(
		ctx context.Context, platformTenantID string, productID string, slug string,
	) (functional.Option[email.Template], error)
	Search(ctx context.Context, q EmailTemplateSearch) (search.Result[email.Template], error)
	Create(
		ctx context.Context, template email.Template,
	) (email.Template, error)
	// Update writes only the envelope fields set on the input, so it never
	// overwrites example_data or the version pointers that SaveExamples and
	// SetVersionPointers change concurrently.
	Update(
		ctx context.Context, in email.UpdateTemplateInput,
	) (functional.Option[email.Template], error)
	// SaveExamples replaces the example_data JSONB column for the given
	// template in a single targeted UPDATE (no full read-then-write).
	SaveExamples(
		ctx context.Context, tenantID string, productID string, templateID string, examples []email.TemplateExample,
	) error
	// SetVersionPointers atomically updates the draft/published version
	// pointers on the template envelope. Used by Publish to swap a draft
	// into the published slot in a single statement.
	SetVersionPointers(
		ctx context.Context, tenantID string, templateID string, draftVersionID *string, publishedVersionID *string,
	) error
	DeleteByPlatformTenantIDAndProductIDAndID(
		ctx context.Context, platformTenantID string, productID string, id string,
	) error
}

// TemplateVersionRepository persists the snapshots of authoring content
// (subject, bodies, variables) attached to a Template. Each Template owns
// at most one DRAFT row and one PUBLISHED row at any time; older PUBLISHED
// rows are moved to ARCHIVED on republish.
//
// email_template_versions has no tenant column: FindByID reads any tenant's
// row, so the caller loads the version only through a template that it
// found with the tenant and product first.
type TemplateVersionRepository interface {
	FindByID(
		ctx context.Context, id string,
	) (functional.Option[email.TemplateVersion], error)
	// FindCurrentDraft returns the template's current DRAFT version, or an
	// absent Option when no draft exists (e.g. immediately after a publish
	// before a new draft has been opened).
	FindCurrentDraft(
		ctx context.Context, templateID string,
	) (functional.Option[email.TemplateVersion], error)
	// FindCurrentPublished returns the template's currently-published
	// version, or an absent Option when the template has never been published.
	FindCurrentPublished(
		ctx context.Context, templateID string,
	) (functional.Option[email.TemplateVersion], error)
	Create(
		ctx context.Context, version email.TemplateVersion,
	) (email.TemplateVersion, error)
	Update(
		ctx context.Context, version email.TemplateVersion,
	) (email.TemplateVersion, error)
	// UpdateStatus transitions a version between DRAFT/PUBLISHED/ARCHIVED.
	// Used by Publish to atomically move the previous published row to
	// ARCHIVED and the draft row to PUBLISHED inside one transaction.
	UpdateStatus(
		ctx context.Context, id string, status email.TemplateVersionStatus, publishedAt *time.Time,
	) error
	// NextVersionNumber returns the next monotonic version number to assign
	// to a new version row for a template.
	NextVersionNumber(
		ctx context.Context, templateID string,
	) (int32, error)
}

// SendRecordRepository persists every send attempt for audit + idempotency.
type SendRecordRepository interface {
	// CreateInternal inserts the record without tenant scoping; used by the
	// dispatcher which already validated tenancy upstream. The "Internal"
	// suffix matches the project convention for internal-only repo methods.
	CreateInternal(
		ctx context.Context, record email.SendRecord,
	) (email.SendRecord, error)
	// Create inserts the record under the given tenant and product.
	Create(
		ctx context.Context, tenantID string, productID string, record email.SendRecord,
	) (email.SendRecord, error)
	// UpdateStatus updates the status of a send record, scoped to the tenant.
	UpdateStatus(
		ctx context.Context, tenantID string, id string, status email.SendStatus, lastError *string, sentAt *time.Time,
	) error
	// FindByPlatformTenantIDAndProductIDAndDedupeKey returns the send record
	// with the dedupe key within the tenant and product.
	FindByPlatformTenantIDAndProductIDAndDedupeKey(
		ctx context.Context, platformTenantID string, productID string, dedupeKey string,
	) (functional.Option[email.SendRecord], error)
	Search(ctx context.Context, q EmailSendSearch) (search.Result[email.SendRecord], error)
	// CountSince returns the number of sends for a product and tenant since
	// the given timestamp.
	CountSince(
		ctx context.Context, tenantID string, productID string, since time.Time,
	) (int64, error)
}
