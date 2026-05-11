//nolint:testpackage // These tests verify encrypted config round-tripping via unexported provider helpers.
package smtp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"anchor/internal/security/encryption"
	serviceconfig "anchor/internal/service/config"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestPrepareUpdatedConfigForStoragePreservesExistingPassword(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(t)
	ctx := context.Background()

	existingConfigJSON, err := provider.PrepareConfigForStorage(ctx, mustMarshalConfig(t, Config{
		Host:        "smtp.anchor.nanostack.dev",
		Port:        defaultPort,
		Encryption:  EncryptionStartTLS,
		AuthMethod:  AuthMethodPlain,
		Username:    "mailer@anchor.nanostack.dev",
		Password:    "existing-secret",
		FromAddress: "noreply@anchor.nanostack.dev",
		FromName:    "Anchor",
	}))
	require.NoError(t, err)

	updatedConfig := mustMarshalConfig(t, Config{
		Host:        "smtp.anchor.nanostack.dev",
		Port:        defaultPort,
		Encryption:  EncryptionStartTLS,
		AuthMethod:  AuthMethodPlain,
		Username:    "updated-mailer@anchor.nanostack.dev",
		FromAddress: "noreply@anchor.nanostack.dev",
		FromName:    "Anchor",
	})

	updatedConfigJSON, err := provider.PrepareUpdatedConfigForStorage(
		ctx,
		existingConfigJSON,
		updatedConfig,
	)
	require.NoError(t, err)

	var existingStored Config
	var updatedStored Config
	require.NoError(t, json.Unmarshal(existingConfigJSON, &existingStored))
	require.NoError(t, json.Unmarshal(updatedConfigJSON, &updatedStored))
	require.Equal(t, existingStored.Password, updatedStored.Password)

	resolved, err := provider.resolveConfig(updatedConfigJSON)
	require.NoError(t, err)
	require.Equal(t, "existing-secret", resolved.Password)
	require.Equal(t, "updated-mailer@anchor.nanostack.dev", resolved.Username)
}

func TestPrepareUpdatedConfigForStorageEncryptsReplacementPassword(t *testing.T) {
	t.Parallel()

	provider := newTestProvider(t)
	ctx := context.Background()

	existingConfigJSON, err := provider.PrepareConfigForStorage(ctx, mustMarshalConfig(t, Config{
		Host:        "smtp.anchor.nanostack.dev",
		Port:        defaultPort,
		Encryption:  EncryptionStartTLS,
		AuthMethod:  AuthMethodPlain,
		Username:    "mailer@anchor.nanostack.dev",
		Password:    "existing-secret",
		FromAddress: "noreply@anchor.nanostack.dev",
	}))
	require.NoError(t, err)

	updatedConfig := mustMarshalConfig(t, Config{
		Host:        "smtp.anchor.nanostack.dev",
		Port:        defaultPort,
		Encryption:  EncryptionStartTLS,
		AuthMethod:  AuthMethodPlain,
		Username:    "mailer@anchor.nanostack.dev",
		Password:    "replacement-secret",
		FromAddress: "noreply@anchor.nanostack.dev",
	})

	updatedConfigJSON, err := provider.PrepareUpdatedConfigForStorage(
		ctx,
		existingConfigJSON,
		updatedConfig,
	)
	require.NoError(t, err)

	var updatedStored Config
	require.NoError(t, json.Unmarshal(updatedConfigJSON, &updatedStored))
	require.NotEqual(t, "replacement-secret", updatedStored.Password)

	resolved, err := provider.resolveConfig(updatedConfigJSON)
	require.NoError(t, err)
	require.Equal(t, "replacement-secret", resolved.Password)
}

func newTestProvider(t *testing.T) *Provider {
	t.Helper()

	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 1)
	}

	encryptionService, err := encryption.NewService(&serviceconfig.CoreConfig{
		Encryption: serviceconfig.EncryptionConfig{
			GlobalKey:        base64.StdEncoding.EncodeToString(rawKey),
			GlobalKeyVersion: "v1",
		},
	})
	require.NoError(t, err)

	return NewProvider(NewProviderParams{
		EncryptionService: encryptionService,
		Logger:            zerolog.Nop(),
	})
}

func mustMarshalConfig(t *testing.T, cfg Config) []byte {
	t.Helper()

	data, err := json.Marshal(cfg)
	require.NoError(t, err)

	return data
}
