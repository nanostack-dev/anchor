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

// LogoutInput carries the bearer access token logout ends the session of. It
// is empty when the request carried none.
type LogoutInput struct {
	AccessToken string
}

// StartSessionInput names an already authenticated platform user to sign in.
type StartSessionInput struct {
	PlatformUserID string `validate:"required"`
	UserID         string `validate:"required"`
	TenantID       string `validate:"required"`
}
