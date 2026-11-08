//go:build !nanogen

package app

import (
	"cmp"
	"context"
	"fmt"
	"os"

	"github.com/nanostack-dev/nanostack-framework/core/config"
	"github.com/nanostack-dev/nanostack-framework/core/nano"

	"anchor/internal/runtimeenv"
)

const (
	// ConfigPathEnv names the YAML file layer. Without it, application.yaml
	// in the working directory is read when it exists.
	ConfigPathEnv = "CONFIG_PATH"
	// EnvironmentEnv is the process environment, which log.environment reads
	// too. core.environment falls back to it (see
	// serviceconfig.CoreConfig.Environment).
	EnvironmentEnv         = "ENVIRONMENT"
	defaultCoreEnvironment = "production"
)

// LoadConfig binds Config. Precedence, highest first: the environment (with
// each *_FILE variable read into its variable first), the YAML file of
// CONFIG_PATH (or ./application.yaml), then the compiled-in defaults.
// core.environment is the exception: the file, then ENVIRONMENT, then
// production.
func LoadConfig(ctx context.Context) (Config, error) {
	if err := runtimeenv.HydrateFileBackedEnv(); err != nil {
		return Config{}, fmt.Errorf("hydrate file-backed env: %w", err)
	}
	file := config.OptionalFile("application.yaml")
	if path := os.Getenv(ConfigPathEnv); path != "" {
		file = config.File(path)
	}
	src, err := config.NewSources(config.YAML("anchor defaults", defaults), file, config.Env())
	if err != nil {
		return Config{}, err
	}
	cfg, err := Bind(ctx, src)
	if err != nil {
		return Config{}, err
	}
	cfg.Core.Environment = cmp.Or(cfg.Core.Environment, os.Getenv(EnvironmentEnv), defaultCoreEnvironment)
	return cfg, nil
}

// Main binds the configuration, builds anchor and runs it until SIGINT or
// SIGTERM, then exits.
func Main() {
	nano.Main(func(ctx context.Context) (*nano.App, error) {
		cfg, err := LoadConfig(ctx)
		if err != nil {
			return nil, err
		}
		return Build(ctx, cfg, Overrides{})
	})
}
