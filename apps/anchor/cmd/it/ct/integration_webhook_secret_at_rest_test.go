package ct_test

import (
	"context"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"
)

func TestWebhookSecretIsEncryptedAtRest(t *testing.T) {
	t.Parallel()
	productContext := createTestProductContext(t)
	instance := createClerkIntegrationInstance(t, productContext)
	updateClerkIntegrationInstance(
		t,
		productContext,
		instance.Id,
		ct.UpdateIntegrationInstanceJSONRequestBody{WebhookSecret: new(clerkTestWebhookSecret)},
	)

	var storedSecret *string
	err := testDB.QueryRowContext(
		context.Background(),
		"SELECT webhook_secret FROM integration_instances WHERE id = $1",
		instance.Id,
	).Scan(&storedSecret)
	require.NoError(t, err)
	require.NotNil(t, storedSecret, "webhook_secret column is NULL after update")

	require.NotEqual(t, clerkTestWebhookSecret, *storedSecret,
		"webhook_secret stored in plaintext at rest: %q", *storedSecret)
	require.NotContains(t, *storedSecret, strings.TrimPrefix(clerkTestWebhookSecret, "whsec_"),
		"webhook_secret key material stored in plaintext at rest: %q", *storedSecret)
}
