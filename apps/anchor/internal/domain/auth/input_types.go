package auth

// RegisterInput defines the input structure for user registration.
type RegisterInput struct {
	Email          string  `validate:"required,email"`
	Password       string  `validate:"required,min=8,strongpassword"`
	InvitationCode *string `validate:"omitempty,notblank"`
	TenantName     *string `validate:"omitempty,min=2,max=100"`
}

// LoginInput defines the input structure for user login.
type LoginInput struct {
	Email    string `validate:"required,email"`
	Password string `validate:"required"`
}

// RefreshTokenInput defines the input structure for token refresh.
type RefreshTokenInput struct {
	RefreshToken string `validate:"required"`
}
