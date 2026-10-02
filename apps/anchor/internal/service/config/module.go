package config

import (
	"errors"

	fxconfig "github.com/nanostack-dev/nanostack-framework/modules/config"

	"go.uber.org/fx"
)

// ProvideCoreConfig loads the core configuration using the FX config loader.
func ProvideCoreConfig(loader fxconfig.Loader) (*CoreConfig, error) {
	var config CoreConfig
	if err := loader.LoadConfig("core", &config); err != nil {
		return nil, err
	}
	return &config, nil
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

// NewModule returns an fx.Option that provides core configuration and specific sub-configurations.
func NewModule() fx.Option {
	return fx.Module(
		"core-config",
		fx.Provide(
			ProvideCoreConfig,
			ProvideAuthConfig,
		),
	)
}
