package invitation

import (
	"github.com/nanostack-dev/shared/toolkit/search"

	"anchor/internal/domain/platform"
)

// CreateInvitationInput defines the input structure for creating an invitation.
type CreateInvitationInput struct {
	TenantID string              `validate:"required,notblank"`
	Email    string              `validate:"required,email"`
	Role     platform.TenantRole `validate:"required"`
}

// DeleteInvitationInput defines the input structure for deleting an invitation.
type DeleteInvitationInput struct {
	TenantID     string `validate:"required,notblank"`
	InvitationID string `validate:"required,notblank"`
}

type SearchPlatformInvitationFilter struct {
	IDs    []string `validate:"omitempty,dive,notblank"`
	Emails []string `validate:"omitempty,dive,email"`
	Code   *string
}

type SortFieldPlatformInvitation string

const (
	SortFieldPlatformInvitationCreatedAt SortFieldPlatformInvitation = "created_at"
	SortFieldPlatformInvitationUpdatedAt SortFieldPlatformInvitation = "updated_at"
	SortFieldPlatformInvitationEmail     SortFieldPlatformInvitation = "email"
)

// SearchInvitationInput defines the input structure for searching invitations.
type SearchInvitationInput struct {
	TenantID string                                                                      `validate:"required,notblank"`
	Request  search.Request[SearchPlatformInvitationFilter, SortFieldPlatformInvitation] `validate:"required"`
}
