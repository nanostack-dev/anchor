// Package config holds anchor's own settings, the core section of the
// service configuration (cmd/app.Config). The env names are the ones the
// deployed stacks and docker-compose set, so each setting names its variable
// instead of taking the one core/config derives from its path. Environment
// is the exception: see CoreConfig.Environment.
package config

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/secret"
)

type CoreConfig struct {
	Auth        AuthConfig        `config:"auth"`
	Encryption  EncryptionConfig  `config:"encryption"`
	Integration IntegrationConfig `config:"integration"`
	// Environment names the deployment: production (or prod), dev,
	// development or test. It selects the production rules, such as
	// https-only event endpoints, and development's insecure refresh cookie.
	// The deploy renderer writes the deployment name here (prod, dev), which
	// differs from the process name in ENVIRONMENT (production,
	// development), so the file layer wins over ENVIRONMENT for this one
	// setting: unset, it is ENVIRONMENT, else production (cmd/app.LoadConfig).
	Environment string `config:"environment,optional"`
}

type AuthConfig struct {
	// RefreshTokenLifetime is the lifetime of a refresh token, in seconds.
	RefreshTokenLifetime int64 `config:"refresh_token_lifetime" env:"REFRESH_TOKEN_LIFETIME" default:"604800" validate:"min=1"`
	// SessionMaxLifetime is the absolute lifetime of a session, in seconds.
	SessionMaxLifetime int64 `config:"session_max_lifetime" env:"SESSION_MAX_LIFETIME" default:"2592000" validate:"min=1"`
	// AccessTokenLifetime is the lifetime of an access token, in seconds.
	AccessTokenLifetime int64 `config:"access_token_lifetime" env:"ACCESS_TOKEN_LIFETIME" default:"3600" validate:"min=1"`
	// AdminJWTSecret signs the platform users' tokens.
	AdminJWTSecret secret.Secret `config:"admin_jwt_secret" env:"ADMIN_JWT_SECRET"`
}

type EncryptionConfig struct {
	// GlobalKey is the base64-encoded 32-byte key that the integration and
	// event endpoint secrets are encrypted with.
	GlobalKey secret.Secret `config:"global_key" env:"APP_ENCRYPTION_KEY"`
	// GlobalKeyVersion labels GlobalKey in the stored ciphertexts.
	GlobalKeyVersion string `config:"global_key_version" env:"APP_ENCRYPTION_KEY_VERSION" default:"v1"`
}

type IntegrationConfig struct {
	// ReconcileScheduleInterval is how often the integration reconcile job
	// is scheduled.
	ReconcileScheduleInterval time.Duration `config:"reconcile_schedule_interval" env:"INTEGRATION_RECONCILE_SCHEDULE_INTERVAL" default:"15m"`
}

func (a AuthConfig) GetAdminJWTSecretAsBytes() []byte {
	return []byte(a.AdminJWTSecret.Reveal())
}

func (c *CoreConfig) IsDevelopment() bool {
	return c.Environment == "development"
}

// IsProduction accepts "prod" because the infra renderer writes the deploy
// environment name, not the process name, into core.environment.
func (c *CoreConfig) IsProduction() bool {
	return c.Environment == "production" || c.Environment == "prod"
}
