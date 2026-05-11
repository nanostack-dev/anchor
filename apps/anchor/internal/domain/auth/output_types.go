package auth

// LoginOutput defines the output structure for a successful login.
type LoginOutput struct {
	AccessToken  string
	RefreshToken string
}
