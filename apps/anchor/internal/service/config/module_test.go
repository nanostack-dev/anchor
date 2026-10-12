package config_test

import (
	"os"
	"path/filepath"
	"testing"

	fxconfig "github.com/nanostack-dev/nanostack-framework/modules/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/service/config"
)

func validAuth() config.AuthConfig {
	return config.AuthConfig{AccessTokenLifetime: 3600, RefreshTokenLifetime: 604800, SessionMaxLifetime: 2592000}
}

func TestProvideAuthConfigAcceptsPositiveLifetimes(t *testing.T) {
	t.Parallel()
	auth, err := config.ProvideAuthConfig(&config.CoreConfig{Auth: validAuth()})
	require.NoError(t, err)
	assert.Equal(t, int64(2592000), auth.SessionMaxLifetime)
}

func TestProvideAuthConfigRefusesAMissingSessionMaxLifetime(t *testing.T) {
	t.Parallel()
	auth := validAuth()
	auth.SessionMaxLifetime = 0
	_, err := config.ProvideAuthConfig(&config.CoreConfig{Auth: auth})
	require.Error(t, err)
}

func TestProvideAuthConfigRefusesAMissingRefreshTokenLifetime(t *testing.T) {
	t.Parallel()
	auth := validAuth()
	auth.RefreshTokenLifetime = 0
	_, err := config.ProvideAuthConfig(&config.CoreConfig{Auth: auth})
	require.Error(t, err)
}

func TestProvideAuthConfigRefusesAMissingAccessTokenLifetime(t *testing.T) {
	t.Parallel()
	auth := validAuth()
	auth.AccessTokenLifetime = 0
	_, err := config.ProvideAuthConfig(&config.CoreConfig{Auth: auth})
	require.Error(t, err)
}

// A deployed environment renders no workflow section: the core config must
// still load, with private workflow targets refused.
func TestProvideCoreConfigLoadsWithoutAWorkflowSection(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "application.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`core:
  environment: production
  auth:
    refresh_token_lifetime: 604800
    session_max_lifetime: 2592000
    access_token_lifetime: 3600
    admin_jwt_secret: secret
  encryption:
    global_key: key
    global_key_version: v1
  integration:
    reconcile_schedule_interval: 15m
`), 0o600))
	loader := fxconfig.NewConfigLoader()
	require.NoError(t, loader.Init(path, t.TempDir()))

	core, err := config.ProvideCoreConfig(loader)

	require.NoError(t, err)
	assert.False(t, core.Workflow.AllowPrivateTargets)
}
