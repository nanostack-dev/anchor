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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestClerkWebhookIdempotencyDuplicateSvixIDIgnored(t *testing.T) {
	// Repeated Clerk deliveries with the same Svix ID are acknowledged without processing the event twice.
	t.Parallel()
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

func TestClerkWebhookConcurrentDuplicateSvixIDsGetTheSequentialDuplicateResponse(t *testing.T) {
	// Reading clerk webhook concurrent duplicate svix IDs the sequential duplicate response succeeds.
	t.Parallel()
	productContext := createTestProductContext(t)
	instance := createActiveClerkIntegrationInstance(t, productContext)

	payload := clerkUserCreatedPayload(
		t, "user_clerk_"+itshared.Faker.UUID().V4(), itshared.Faker.Internet().Email(), "Concurrent", "Delivery",
	)
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	svixID := "msg_test_concurrent_" + itshared.Faker.UUID().V4()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	headerEditor := fixedSvixHeaderEditor(t, payloadBytes, clerkTestWebhookSecret, svixID, timestamp)
	deliver := func() (int, string, string) {
		resp, sendErr := testTenant(t).NoAuthClient.IngestWebhookWithBodyWithResponse(
			t.Context(),
			productContext.ProductID,
			"CLERK",
			"application/json",
			io.NopCloser(bytes.NewReader(payloadBytes)),
			headerEditor,
		)
		if sendErr != nil || resp.JSON200 == nil {
			if sendErr != nil {
				return 0, "", sendErr.Error()
			}
			return resp.StatusCode(), "", string(resp.Body)
		}
		return resp.StatusCode(), resp.JSON200.EventId, ""
	}

	const deliveries = 12
	statuses := make([]int, deliveries)
	eventIDs := make([]string, deliveries)
	bodies := make([]string, deliveries)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for delivery := range deliveries {
		wg.Go(func() {
			<-start
			statuses[delivery], eventIDs[delivery], bodies[delivery] = deliver()
		})
	}
	close(start)
	wg.Wait()

	sequentialStatus, sequentialEventID, sequentialBody := deliver()
	require.Equal(t, http.StatusOK, sequentialStatus, sequentialBody)
	for delivery := range deliveries {
		assert.Equal(t, http.StatusOK, statuses[delivery], bodies[delivery])
		assert.Equal(t, sequentialEventID, eventIDs[delivery])
	}

	var stored int
	require.NoError(t, testDB.QueryRowContext(t.Context(),
		`SELECT count(*) FROM integration_events WHERE integration_instance_id = $1 AND external_event_id = $2`,
		instance.Id, svixID,
	).Scan(&stored))
	assert.Equal(t, 1, stored)
}

func TestClerkWebhookSignatureValidationBadSignatureReturns401(t *testing.T) {
	// Clerk webhook signature validation bad signature returns 401.
	t.Parallel()
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
	// Clerk webhook signature validation missing svix ID returns 401.
	t.Parallel()
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
