package platform

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

// SortFieldPlatformUser is the sort_by type of POST /platform-users/search. The
// generated search owns the values; this type gives the OpenAPI schema its Go type.
type SortFieldPlatformUser string

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
