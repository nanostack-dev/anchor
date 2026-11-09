package invitation

import (
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

// SortFieldPlatformInvitation is the sort_by type of POST /platform-invitations/search.
// The generated search owns the values; this type gives the OpenAPI schema its Go type.
type SortFieldPlatformInvitation string
