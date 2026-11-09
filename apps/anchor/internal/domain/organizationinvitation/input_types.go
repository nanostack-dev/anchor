package organizationinvitation

import (
	"time"
)

type CreateInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	Email          string `validate:"required,email"`
	// Nil invites a member who will hold no role.
	RoleID    *string `validate:"omitempty,notblank"`
	ExpiresAt *time.Time
}

type GetInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	InvitationID   string `validate:"required,notblank"`
}

type UpdateInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	InvitationID   string `validate:"required,notblank"`
	// Nil sets the invitation to no role.
	RoleID    *string   `validate:"omitempty,notblank"`
	ExpiresAt time.Time `validate:"required"`
}

type DeleteInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	InvitationID   string `validate:"required,notblank"`
}

type AcceptInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	InvitationID   string `validate:"required,notblank"`
	ProductUserID  string `validate:"required,notblank"`
}

// SortField is the sort_by type of the organization invitation searches. The
// generated searches own the values; this type gives the OpenAPI schema its Go type.
type SortField string
