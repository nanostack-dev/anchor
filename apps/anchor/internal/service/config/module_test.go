package config_test

import (
	"testing"

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
