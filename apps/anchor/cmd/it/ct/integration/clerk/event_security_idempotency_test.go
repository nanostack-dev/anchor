package ct_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestClerkWebhookIdempotencyDuplicateSvixIDIgnored(t *testing.T) {
	productContext := createTestProductContext(t)
	createActiveClerkIntegrationInstance(t, productContext)

	externalID := "user_clerk_" + itshared.Faker.UUID().V4()
	email := itshared.Faker.Internet().Email()
	payload := clerkUserCreatedPayload(t, externalID, email, "Idempotent", "User")
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	svixID := "msg_test_idempotent_" + itshared.Faker.UUID().V4()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	headerEditor := fixedSvixHeaderEditor(t, payloadBytes, clerkTestWebhookSecret, svixID, timestamp)

	resp1, err1 := testTenant(t).NoAuthClient.IngestWebhookWithBodyWithResponse(
		context.Background(),
		productContext.ProductID,
		"CLERK",
		"application/json",
		io.NopCloser(bytes.NewReader(payloadBytes)),
		headerEditor,
	)
	require.NoError(t, err1)
	assert.Equal(t, http.StatusOK, resp1.StatusCode())

	resp2, err2 := testTenant(t).NoAuthClient.IngestWebhookWithBodyWithResponse(
		context.Background(),
		productContext.ProductID,
		"CLERK",
		"application/json",
		io.NopCloser(bytes.NewReader(payloadBytes)),
		headerEditor,
	)
	require.NoError(t, err2)
	assert.Equal(t, http.StatusOK, resp2.StatusCode())

	require.Eventually(t, func() bool {
		searchResp := searchProductUsers(t, productContext)
		return searchResp.JSON200.Total == 1
	}, 10*time.Second, 200*time.Millisecond)
}

func TestClerkWebhookSignatureValidationBadSignatureReturns401(t *testing.T) {
	productContext := createTestProductContext(t)
	createActiveClerkIntegrationInstance(t, productContext)

	externalID := "user_clerk_" + itshared.Faker.UUID().V4()
	payload := clerkUserCreatedPayload(t, externalID, itshared.Faker.Internet().Email(), "John", "Doe")

	wrongSecret := "whsec_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	resp := sendClerkWebhook(t, productContext.ProductID, payload, wrongSecret)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())

	require.Eventually(t, func() bool {
		searchResp := searchProductUsers(t, productContext)
		return searchResp.JSON200.Total == 0
	}, 5*time.Second, 200*time.Millisecond)
}

// TestClerkWebhookSignatureValidationMissingSvixIDReturns401 covers a missing
// svix-id header. Svix signs over svix-id, svix-timestamp, and the body, so
// removing the header invalidates the signature and the request never reaches
// the event-id check. A signature that does not verify is a credential that
// failed to authenticate, so this is a 401.
func TestClerkWebhookSignatureValidationMissingSvixIDReturns401(t *testing.T) {
	productContext := createTestProductContext(t)
	createActiveClerkIntegrationInstance(t, productContext)

	externalID := "user_clerk_" + itshared.Faker.UUID().V4()
	payload := clerkUserCreatedPayload(t, externalID, itshared.Faker.Internet().Email(), "No", "Header")
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	msgID := "msg_test_missing_id_" + itshared.Faker.UUID().V4()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	headerEditor := fixedSvixHeaderEditorWithoutID(t, payloadBytes, clerkTestWebhookSecret, msgID, timestamp)

	resp, sendErr := testTenant(t).NoAuthClient.IngestWebhookWithBodyWithResponse(
		context.Background(),
		productContext.ProductID,
		"CLERK",
		"application/json",
		io.NopCloser(bytes.NewReader(payloadBytes)),
		headerEditor,
	)
	require.NoError(t, sendErr)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())

	require.Eventually(t, func() bool {
		searchResp := searchProductUsers(t, productContext)
		return searchResp.JSON200.Total == 0
	}, 5*time.Second, 200*time.Millisecond)
}

func fixedSvixHeaderEditor(
	t *testing.T,
	payload []byte,
	secret string,
	msgID string,
	timestamp string,
) func(context.Context, *http.Request) error {
	t.Helper()

	secretKey := secret
	if len(secretKey) > 6 && secretKey[:6] == "whsec_" {
		secretKey = secretKey[6:]
	}
	secretBytes, err := base64.StdEncoding.DecodeString(secretKey)
	require.NoError(t, err)

	toSign := fmt.Sprintf("%s.%s.%s", msgID, timestamp, string(payload))
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(toSign))
	signature := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Svix-Id", msgID)
		req.Header.Set("Svix-Timestamp", timestamp)
		req.Header.Set("Svix-Signature", signature)
		return nil
	}
}

func fixedSvixHeaderEditorWithoutID(
	t *testing.T,
	payload []byte,
	secret string,
	msgID string,
	timestamp string,
) func(context.Context, *http.Request) error {
	t.Helper()

	secretKey := secret
	if len(secretKey) > 6 && secretKey[:6] == "whsec_" {
		secretKey = secretKey[6:]
	}
	secretBytes, err := base64.StdEncoding.DecodeString(secretKey)
	require.NoError(t, err)

	toSign := fmt.Sprintf("%s.%s.%s", msgID, timestamp, string(payload))
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(toSign))
	signature := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Svix-Timestamp", timestamp)
		req.Header.Set("Svix-Signature", signature)
		return nil
	}
}
