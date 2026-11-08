package app_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	genconfig "github.com/nanostack-dev/nanostack-framework/gen/config"
	"github.com/nanostack-dev/nanostack-framework/gen/wire"

	"anchor/cmd/app"
)

// TestGeneratedFilesAreFresh fails when wire.gen.go, config.gen.go,
// CONFIG.md or .env.example no longer match the manifest and Config: run
// go generate ./cmd/app.
func TestGeneratedFilesAreFresh(t *testing.T) {
	require.NoError(t, wire.Run(wire.Options{Dir: ".", Out: ".", Check: true}))
	require.NoError(t, genconfig.Run(genconfig.Options{Dir: ".", EnvExample: "../../.env.example", Check: true}))
}

// setRequired sets the settings that have no default: the database address
// and the secrets.
func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv(app.ConfigPathEnv, "testdata/empty.yaml")
	for name, value := range map[string]string{
		"POSTGRES_HOST":      "db",
		"POSTGRES_USER":      "anchor",
		"POSTGRES_DB":        "anchor",
		"ADMIN_JWT_SECRET":   "jwt",
		"APP_ENCRYPTION_KEY": "key",
		"ALLOWED_ORIGIN":     "https://app.example",
	} {
		t.Setenv(name, value)
	}
}

func TestLoadConfigKeepsTheDeployedEnvNamesAndDefaults(t *testing.T) {
	setRequired(t)
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("REDIS_ADDRESS", "redis:6379")
	t.Setenv("REFRESH_TOKEN_LIFETIME", "60")
	t.Setenv("INTEGRATION_RECONCILE_SCHEDULE_INTERVAL", "1h")
	t.Setenv("APP_ENCRYPTION_KEY_VERSION", "v2")
	t.Setenv("ENVIRONMENT", "development")

	cfg, err := app.LoadConfig(t.Context())
	require.NoError(t, err)

	assert.Equal(t, "db", cfg.Postgres.Host)
	assert.Equal(t, "anchor", cfg.Postgres.DB)
	assert.Equal(t, 5432, cfg.Postgres.Port)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "https://app.example", cfg.Server.AllowedOrigin)
	assert.Equal(t, 5*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, "redis:6379", cfg.Redis.Address)
	assert.Equal(t, int64(60), cfg.Core.Auth.RefreshTokenLifetime)
	assert.Equal(t, int64(3600), cfg.Core.Auth.AccessTokenLifetime)
	assert.Equal(t, "jwt", cfg.Core.Auth.AdminJWTSecret.Reveal())
	assert.Equal(t, "key", cfg.Core.Encryption.GlobalKey.Reveal())
	assert.Equal(t, "v2", cfg.Core.Encryption.GlobalKeyVersion)
	assert.Equal(t, time.Hour, cfg.Core.Integration.ReconcileScheduleInterval)
	assert.Equal(t, "development", cfg.Core.Environment)
	assert.Equal(t, "development", cfg.Log.Environment)
	assert.True(t, cfg.Migrations.Enabled)
	assert.Equal(t, 15*time.Second, cfg.App.StopTimeout)
}

func TestLoadConfigDefaultsToTheLegacyLogWire(t *testing.T) {
	setRequired(t)

	cfg, err := app.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "legacy", cfg.Log.Wire)

	t.Setenv("LOG_WIRE", "slog")
	cfg, err = app.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "slog", cfg.Log.Wire)
}

// core.environment holds the deployment name the renderer writes (prod,
// dev); ENVIRONMENT holds the process name (production, development). The
// file wins, then ENVIRONMENT, then production.
func TestLoadConfigCoreEnvironment(t *testing.T) {
	setRequired(t)
	cfg, err := app.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "production", cfg.Core.Environment, "neither set")

	t.Setenv("ENVIRONMENT", "development")
	cfg, err = app.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "development", cfg.Core.Environment, "ENVIRONMENT only")

	t.Setenv(app.ConfigPathEnv, "testdata/dev-deploy.yaml")
	cfg, err = app.LoadConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "dev", cfg.Core.Environment, "the file wins")
	assert.Equal(t, "development", cfg.Log.Environment)
}

func TestLoadConfigRefusesASecretInTheFile(t *testing.T) {
	setRequired(t)
	t.Setenv(app.ConfigPathEnv, "testdata/secret-in-file.yaml")
	_, err := app.LoadConfig(t.Context())
	require.ErrorContains(t, err, "must not be in a config file")
}

// The suites' and the image's file layers bind with only the values their
// launchers set in the environment.
func TestCommittedFileLayersBind(t *testing.T) {
	for _, path := range []string{
		"../../application.yaml",
		"../it/contract/application.yaml",
		"../it/ct/application.yaml",
		"../it/service/application.yaml",
		"../../../../anchor-ui/e2e/runtime/application.yaml",
	} {
		t.Run(path, func(t *testing.T) {
			setRequired(t)
			t.Setenv(app.ConfigPathEnv, path)
			_, err := app.LoadConfig(t.Context())
			require.NoError(t, err)
		})
	}
}
