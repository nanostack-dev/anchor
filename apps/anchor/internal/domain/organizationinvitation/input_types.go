package organizationinvitation

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

type CreateInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	Email          string `validate:"required,email"`
	RoleID         string `validate:"required,notblank"`
	ExpiresAt      *time.Time
}

type GetInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	InvitationID   string `validate:"required,notblank"`
}

type UpdateInput struct {
	ProductID      string    `validate:"required,notblank"`
	OrganizationID string    `validate:"required,notblank"`
	InvitationID   string    `validate:"required,notblank"`
	RoleID         string    `validate:"required,notblank"`
	ExpiresAt      time.Time `validate:"required"`
}

type DeleteInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	InvitationID   string `validate:"required,notblank"`
}

type ResendInput struct {
	ProductID      string `validate:"required,notblank"`
	OrganizationID string `validate:"required,notblank"`
	InvitationID   string `validate:"required,notblank"`
}

type LookupInput struct {
	ProductID string `validate:"required,notblank"`
	Token     string `validate:"required,notblank"`
}

type AcceptInput struct {
	ProductID     string `validate:"required,notblank"`
	Token         string `validate:"required,notblank"`
	ProductUserID string `validate:"required,notblank"`
}

type SearchFilter struct {
	Statuses []Status `validate:"omitempty,dive,oneof=pending accepted expired"`
}

type SortField string

const (
	SortFieldCreatedAt SortField = "created_at"
	SortFieldEmail     SortField = "email"
	SortFieldExpiresAt SortField = "expires_at"
)

type SearchInput struct {
	ProductID      string                                  `validate:"required,notblank"`
	OrganizationID string                                  `validate:"required,notblank"`
	Request        search.Request[SearchFilter, SortField] `validate:"required"`
}
