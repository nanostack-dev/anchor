package platform

import (
	"github.com/nanostack-dev/shared/toolkit/search"
)

// DeletePlatformUserInput defines the input structure for deleting a platform user.
type DeletePlatformUserInput struct {
	TenantID       string `validate:"required,notblank"`
	PlatformUserID string `validate:"required,notblank"`
}

// GetPlatformUserInput defines the input structure for retrieving a platform user.
type GetPlatformUserInput struct {
	TenantID       string `validate:"required,notblank"`
	PlatformUserID string `validate:"required,notblank"`
}

// SearchPlatformUserFilter defines search criteria for platform users.
type SearchPlatformUserFilter struct {
	IDs    []string `validate:"omitempty,dive,notblank"`
	Emails []string `validate:"omitempty,dive,email"`
	Roles  []TenantRole
}

type SortFieldPlatformUser string

const (
	SortFieldPlatformUserCreatedAt SortFieldPlatformUser = "created_at"
	SortFieldPlatformUserUpdatedAt SortFieldPlatformUser = "updated_at"
	SortFieldPlatformUserEmail     SortFieldPlatformUser = "email"
	SortFieldPlatformUserRole      SortFieldPlatformUser = "role"
)

// SearchPlatformUsersInput defines the input structure for searching platform users.
type SearchPlatformUsersInput struct {
	TenantID string                                                          `validate:"required,notblank"`
	Request  search.Request[SearchPlatformUserFilter, SortFieldPlatformUser] `validate:"required"`
}

// GetPlatformUserByUserIDInput defines the input structure for retrieving a platform user by user Name.
type GetPlatformUserByUserIDInput struct {
	TenantID string `validate:"required,notblank"`
	UserID   string `validate:"required,notblank"`
}

// InvitePlatformUserInput defines the input structure for inviting a platform user.
type InvitePlatformUserInput struct {
	Email string     `validate:"required,email"`
	Role  TenantRole `validate:"required"`
}
