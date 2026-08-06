package organization

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

type CreateOrganizationInput struct {
	ProductID   string  `validate:"required,notblank"`
	Name        string  `validate:"required,notblank,min=2,max=100"`
	Description *string `validate:"omitempty,max=500"`
	// Metadata is the caller-supplied key-value metadata. Nil leaves it unset.
	Metadata map[string]any
}

type FindOrganizationInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
}

type UpdateOrganizationInput struct {
	ProductID      string  `validate:"required,notblank"`
	OrganizationID string  `validate:"required,notblank"`
	Name           *string `validate:"omitempty,notblank,min=2,max=100"`
	Description    *string `validate:"omitempty,max=500"`
	// Metadata replaces the stored metadata wholesale, matching the
	// full-replace semantics the PUT endpoint already applies to Description.
	// Nil clears it.
	Metadata map[string]any
}

type DeleteOrganizationInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
}
type SearchProductOrganizationFilter struct {
	IDs   []string `validate:"omitempty,dive,notblank"`
	Names []string `validate:"omitempty,dive,notblank"`
}

type SortFieldProductOrganization string

const (
	SortFieldProductOrganizationCreatedAt SortFieldProductOrganization = "created_at"
	SortFieldProductOrganizationUpdatedAt SortFieldProductOrganization = "updated_at"
	SortFieldProductOrganizationName      SortFieldProductOrganization = "name"
)

type SearchProductOrganizationsInput struct {
	ProductID string                                                                        `validate:"required,notblank"`
	Request   search.Request[SearchProductOrganizationFilter, SortFieldProductOrganization] `validate:"required"`
}

// CreateOrganizationWithMemberInput is the input for atomically creating an organization
// and assigning its founding member with a role in a single transaction.
type CreateOrganizationWithMemberInput struct {
	ProductID     string  `validate:"required,notblank"`
	Name          string  `validate:"required,notblank,min=2,max=100"`
	Description   *string `validate:"omitempty,max=500"`
	ProductUserID string  `validate:"required,notblank"`
	RoleID        string  `validate:"required,notblank"`
	// Metadata is the caller-supplied key-value metadata. Nil leaves it unset.
	Metadata map[string]any
}

// OrganizationWithMemberResult is the result of CreateWithMember, containing the
// created (or already-existing) organization and its founding membership.
// WasExisting is true when the call was idempotent (user already had a membership).
type OrganizationWithMemberResult struct { //nolint:revive // name keeps service API intent clear
	Organization Organization
	Membership   Membership
	WasExisting  bool
}
