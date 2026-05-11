//nolint:testpackage // tests require access to unexported symbols (resolveConfig, newTestProvider)
package clerk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"anchor/internal/security/encryption"
	serviceconfig "anchor/internal/service/config"

	"github.com/nanostack-dev/shared/toolkit"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestPrepareConfigForStorageEncryptsAPIKey(t *testing.T) {
	p := newTestProvider(t)

	normalized, err := p.PrepareConfigForStorage(
		context.Background(),
		[]byte(`{"api_key":"sk_test_123"}`),
	)
	require.NoError(t, err)

	var stored Config
	require.NoError(t, json.Unmarshal(normalized, &stored))
	require.NotEqual(t, "sk_test_123", stored.APIKey)
	require.True(t, toolkit.IsVersionedEncryptedSecret(stored.APIKey))

	resolved, err := p.resolveConfig(normalized)
	require.NoError(t, err)
	require.Equal(t, "sk_test_123", resolved.APIKey)
}

func TestPrepareConfigForStorageDoesNotDoubleEncrypt(t *testing.T) {
	p := newTestProvider(t)

	// First pass
	normalized1, err := p.PrepareConfigForStorage(
		context.Background(),
		[]byte(`{"api_key":"sk_test_123"}`),
	)
	require.NoError(t, err)

	// Second pass (e.g. updating some other config value, while keeping the encrypted API key)
	normalized2, err := p.PrepareConfigForStorage(
		context.Background(),
		normalized1,
	)
	require.NoError(t, err)

	// Ensure the second pass doesn't modify the already encrypted config
	require.Equal(t, string(normalized1), string(normalized2))

	// Ensure it still resolves correctly
	resolved, err := p.resolveConfig(normalized2)
	require.NoError(t, err)
	require.Equal(t, "sk_test_123", resolved.APIKey)
}

func TestPrepareConfigForStorageRequiresEncryptionKeyWhenAPIKeyProvided(t *testing.T) {
	p := &Provider{}

	_, err := p.PrepareConfigForStorage(
		context.Background(),
		[]byte(`{"api_key":"sk_test_123"}`),
	)
	require.ErrorIs(t, err, errMissingEncryptionKey)
}

func TestReconcileBuildsCommandsFromPaginatedClerkUsers(t *testing.T) {
	t.Parallel()

	apiKey := "sk_test_abc"
	wiremockURL := startWireMock(t)

	p := newTestProvider(t)
	p.baseURL = wiremockURL + "/v1"

	commands, err := p.Reconcile(
		context.Background(),
		[]byte(`{"api_key":"`+apiKey+`"}`),
	)
	require.NoError(t, err)
	require.Len(t, commands, 3)

	for _, cmd := range commands {
		require.Equal(t, CommandUpsertUser, cmd.Type)
		upsertData, ok := cmd.Data.(UpsertUserData)
		require.True(t, ok)
		require.NotEmpty(t, upsertData.ExternalID)
		require.NotEmpty(t, upsertData.Email)
	}
}

func startWireMock(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	wd, err := os.Getwd()
	require.NoError(t, err)
	mockDir := filepath.Join(wd, "testdata", "wiremock")

	req := testcontainers.ContainerRequest{
		Image:        "wiremock/wiremock:3.3.1",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor: wait.ForHTTP("/__admin/health").
			WithPort("8080/tcp").
			WithStartupTimeout(30 * time.Second),
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.Binds = []string{mockDir + ":/home/wiremock:ro"}
		},
	}

	wm, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, testcontainers.TerminateContainer(wm))
	})

	host, err := wm.Host(ctx)
	require.NoError(t, err)
	port, err := wm.MappedPort(ctx, "8080/tcp")
	require.NoError(t, err)

	return "http://" + net.JoinHostPort(host, port.Port())
}

func newTestProvider(t *testing.T) *Provider {
	t.Helper()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}

	encService, err := encryption.NewService(&serviceconfig.CoreConfig{
		Encryption: serviceconfig.EncryptionConfig{
			GlobalKey:        base64.StdEncoding.EncodeToString(key),
			GlobalKeyVersion: "v1",
		},
	})
	require.NoError(t, err)

	return NewProvider(NewProviderParams{
		EncryptionService: encService,
		Logger:            zerolog.Nop(),
	})
}
