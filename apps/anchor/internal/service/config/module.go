package config

import (
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

// ProvideAuthConfig extracts AuthConfig from CoreConfig.
func ProvideAuthConfig(coreCfg *CoreConfig) AuthConfig {
	return coreCfg.Auth
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
