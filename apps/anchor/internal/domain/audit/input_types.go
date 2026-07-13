package audit

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

// SearchFilter defines filterable fields when searching audit logs.
type SearchFilter struct {
	OrganizationID *string     `validate:"omitempty,notblank"`
	Actions        []string    `validate:"omitempty,dive,notblank"`
	ActorTypes     []ActorType `validate:"omitempty,dive,notblank"`
	ActorID        *string     `validate:"omitempty,notblank"`
	TargetType     *string     `validate:"omitempty,notblank"`
	TargetID       *string     `validate:"omitempty,notblank"`
	Outcome        *Outcome    `validate:"omitempty,notblank"`
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
}

// SortField defines valid sort fields for audit log search results.
type SortField string

// SortFieldCreatedAt sorts by entry creation time (the only supported field).
const SortFieldCreatedAt SortField = "created_at"

// SearchInput is the input for searching audit logs within a product.
type SearchInput struct {
	TenantID  string                                  `validate:"required,notblank"`
	ProductID string                                  `validate:"required,notblank"`
	Request   search.Request[SearchFilter, SortField] `validate:"required"`
}
