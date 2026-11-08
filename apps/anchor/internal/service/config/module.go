package config

import (
	"errors"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module provides the core configuration to the code that shares one value
// of it, and the auth settings.
var Module = nano.Module("core-config", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(NewCoreConfig, ProvideAuthConfig),
)

// NewCoreConfig shares the core section of the bound configuration.
func NewCoreConfig(cfg CoreConfig) *CoreConfig {
	return new(cfg)
}

// ProvideAuthConfig extracts AuthConfig from CoreConfig. A missing lifetime
// fails startup: read as zero, it would expire every token at issue.
func ProvideAuthConfig(coreCfg *CoreConfig) (AuthConfig, error) {
	auth := coreCfg.Auth
	if auth.AccessTokenLifetime <= 0 || auth.RefreshTokenLifetime <= 0 || auth.SessionMaxLifetime <= 0 {
		return AuthConfig{}, errors.New(
			"core.auth access_token_lifetime, refresh_token_lifetime and session_max_lifetime must be positive",
		)
	}
	return auth, nil
}
