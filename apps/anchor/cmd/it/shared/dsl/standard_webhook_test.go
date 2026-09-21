package itdsl_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/events"

	"github.com/stretchr/testify/require"
	svix "github.com/svix/svix-webhooks/go"
)

func TestAssertStandardWebhookAcceptsSignedDelivery(t *testing.T) {
	t.Parallel()

	secret, err := events.NewSigningSecret()
	require.NoError(t, err)
	body := []byte(
		`{"type":"organization.created","timestamp":"2026-09-01T00:00:00.000000000Z","data":{"organization_id":"org_1"}}`,
	)
	itdsl.AssertStandardWebhook(t, secret, mustSignedDelivery(t, secret, "pevt_teststandardwebhook01", body))
}

func mustSignedDelivery(t *testing.T, secret, msgID string, body []byte) itdsl.StandardWebhookDelivery {
	t.Helper()
	now := time.Now()
	wh, err := svix.NewWebhook(secret)
	require.NoError(t, err)
	signature, err := wh.Sign(msgID, now, body)
	require.NoError(t, err)
	headers := make(http.Header, 4)
	headers.Set("Content-Type", "application/json")
	headers.Set("Webhook-Id", msgID)
	headers.Set("Webhook-Timestamp", strconv.FormatInt(now.Unix(), 10))
	headers.Set("Webhook-Signature", signature)
	return itdsl.StandardWebhookDelivery{
		Method:  http.MethodPost,
		Headers: headers,
		Body:    body,
	}
}
