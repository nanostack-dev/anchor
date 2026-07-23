package webhooks_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/webhook"
)

const receiverURL = "https://receiver.example.com/hooks/anchor"

func TestWebhookEndpointCRUD(t *testing.T) {
	ctx := context.Background()
	productContext := createTestProductContext(t)
	client := productContext.OwnerAuthenticatedClient()

	created := createEndpoint(t, productContext, receiverURL, []string{"license.*"})

	t.Run("CreateReturnsTheSecretExactlyOnce", func(t *testing.T) {
		assert.NotEmpty(t, created.Endpoint.Id)
		assert.Equal(t, productContext.ProductID, created.Endpoint.ProductId)
		assert.Equal(t, receiverURL, created.Endpoint.Url)
		assert.Equal(t, []string{"license.*"}, created.Endpoint.EventTypes)
		assert.Equal(t, ct.WebhookEndpointStatusENABLED, created.Endpoint.Status)
		assert.Equal(t, 0, created.Endpoint.ConsecutiveFailureCount)

		require.True(
			t, strings.HasPrefix(created.Secret, webhook.SigningPrefix),
			"the signing secret carries its identifying prefix",
		)
	})

	t.Run("TheSecretNeverAppearsInGetOrList", func(t *testing.T) {
		getResp, err := client.GetWebhookEndpointWithResponse(
			ctx, productContext.ProductID, created.Endpoint.Id,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, getResp.StatusCode())
		require.NotNil(t, getResp.JSON200)
		assert.Equal(t, created.Endpoint.Id, getResp.JSON200.Id)
		assertBodyHidesSecret(t, getResp.Body, created.Secret)

		listResp, err := client.ListWebhookEndpointsWithResponse(ctx, productContext.ProductID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, listResp.StatusCode())
		require.NotNil(t, listResp.JSON200)
		assert.Len(t, listResp.JSON200.Items, 1)
		assertBodyHidesSecret(t, listResp.Body, created.Secret)
	})

	t.Run("PatchUpdatesUrlDescriptionAndSubscriptions", func(t *testing.T) {
		updatedURL := "https://receiver.example.com/hooks/v2"
		resp, err := client.UpdateWebhookEndpointWithResponse(
			ctx, productContext.ProductID, created.Endpoint.Id,
			ct.UpdateWebhookEndpointJSONRequestBody{
				Url:         &updatedURL,
				Description: new("updated"),
				EventTypes:  &[]string{webhook.EventTypeLicenseRevoked, "plan.*"},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		assert.Equal(t, updatedURL, resp.JSON200.Url)
		assert.Equal(t, "updated", *resp.JSON200.Description)
		assert.ElementsMatch(
			t, []string{webhook.EventTypeLicenseRevoked, "plan.*"}, resp.JSON200.EventTypes,
		)
	})

	t.Run("DeleteRemovesTheEndpoint", func(t *testing.T) {
		doomed := createEndpoint(
			t, productContext, "https://receiver.example.com/doomed", []string{"license.*"},
		)

		deleteResp, err := client.DeleteWebhookEndpointWithResponse(
			ctx, productContext.ProductID, doomed.Endpoint.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode())

		getResp, err := client.GetWebhookEndpointWithResponse(
			ctx, productContext.ProductID, doomed.Endpoint.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode())
	})
}

// assertBodyHidesSecret checks the raw response bytes, not just the decoded
// struct: a schema that accidentally grew a secret field would still pass a
// field-by-field assertion.
func assertBodyHidesSecret(t *testing.T, body []byte, secret string) {
	t.Helper()
	assert.NotContains(t, string(body), secret, "the signing secret must never be readable back")
	assert.NotContains(t, string(body), webhook.SigningPrefix)
}

func TestWebhookEndpointValidation(t *testing.T) {
	ctx := context.Background()
	productContext := createTestProductContext(t)
	client := productContext.OwnerAuthenticatedClient()

	tests := []struct {
		name       string
		url        string
		eventTypes []string
	}{
		{
			name:       "an unknown event type is rejected",
			url:        receiverURL,
			eventTypes: []string{"billing.charged"},
		},
		{
			name:       "an unknown wildcard group is rejected",
			url:        receiverURL,
			eventTypes: []string{"billing.*"},
		},
		{
			name:       "an empty subscription list is rejected",
			url:        receiverURL,
			eventTypes: []string{},
		},
		{
			name:       "a non-http url is rejected",
			url:        "ftp://receiver.example.com/hooks",
			eventTypes: []string{"license.*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.CreateWebhookEndpointWithResponse(
				ctx, productContext.ProductID,
				ct.CreateWebhookEndpointJSONRequestBody{
					Url:        tt.url,
					EventTypes: tt.eventTypes,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
		})
	}
}

func TestWebhookEndpointEnableDisable(t *testing.T) {
	ctx := context.Background()
	productContext := createTestProductContext(t)
	client := productContext.OwnerAuthenticatedClient()

	created := createEndpoint(
		t, productContext, "https://receiver.example.com/toggle", []string{"license.*"},
	)

	disableResp, disableErr := client.DisableWebhookEndpointWithResponse(
		ctx, productContext.ProductID, created.Endpoint.Id,
	)
	require.NoError(t, disableErr)
	require.Equal(t, http.StatusOK, disableResp.StatusCode())
	require.NotNil(t, disableResp.JSON200)
	assert.Equal(t, ct.WebhookEndpointStatusDISABLED, disableResp.JSON200.Status)
	require.NotNil(t, disableResp.JSON200.DisabledReason)

	t.Run("APingOnADisabledEndpointIsRejected", func(t *testing.T) {
		pingResp, err := client.PingWebhookEndpointWithResponse(
			ctx, productContext.ProductID, created.Endpoint.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, pingResp.StatusCode())
	})

	enableResp, enableErr := client.EnableWebhookEndpointWithResponse(
		ctx, productContext.ProductID, created.Endpoint.Id,
	)
	require.NoError(t, enableErr)
	require.Equal(t, http.StatusOK, enableResp.StatusCode())
	require.NotNil(t, enableResp.JSON200)
	assert.Equal(t, ct.WebhookEndpointStatusENABLED, enableResp.JSON200.Status)
	assert.Nil(t, enableResp.JSON200.DisabledReason, "re-enabling clears the disable reason")
	assert.Equal(t, 0, enableResp.JSON200.ConsecutiveFailureCount)
}

func TestWebhookSecretRotation(t *testing.T) {
	ctx := context.Background()
	productContext := createTestProductContext(t)
	client := productContext.OwnerAuthenticatedClient()

	created := createEndpoint(
		t, productContext, "https://receiver.example.com/rotate", []string{"license.*"},
	)

	rotateResp, err := client.RotateWebhookEndpointSecretWithResponse(
		ctx, productContext.ProductID, created.Endpoint.Id,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rotateResp.StatusCode())
	require.NotNil(t, rotateResp.JSON200)

	assert.NotEqual(t, created.Secret, rotateResp.JSON200.Secret)
	assert.True(t, strings.HasPrefix(rotateResp.JSON200.Secret, webhook.SigningPrefix))

	// Both secrets must sign until the old one expires, so a receiver can roll
	// over without coordination or downtime.
	stored, err := SecretRepo.ListByEndpointInternal(t.Context(), created.Endpoint.Id)
	require.NoError(t, err)
	require.Len(t, stored, 2)

	usable := webhook.UsableSecrets(stored, time.Now())
	assert.Len(t, usable, 2, "both sides of a live rotation still sign")

	var active, expiring int
	for _, secret := range stored {
		switch secret.Status {
		case webhook.SecretStatusActive:
			active++
		case webhook.SecretStatusExpiring:
			expiring++
			require.NotNil(t, secret.ExpiresAt)
			assert.WithinDuration(
				t, time.Now().Add(webhook.SecretRotationGrace), *secret.ExpiresAt, time.Minute,
			)
		}
	}
	assert.Equal(t, 1, active)
	assert.Equal(t, 1, expiring)
}

func TestWebhookEventTypeCatalog(t *testing.T) {
	ctx := context.Background()
	productContext := createTestProductContext(t)

	resp, err := productContext.OwnerAuthenticatedClient().
		ListWebhookEventTypesWithResponse(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())
	require.NotNil(t, resp.JSON200)

	assert.Equal(t, webhook.APIVersion, resp.JSON200.ApiVersion)
	require.NotEmpty(t, resp.JSON200.Items)

	types := make([]string, 0, len(resp.JSON200.Items))
	for _, item := range resp.JSON200.Items {
		assert.NotEmpty(t, item.Group)
		assert.NotEmpty(t, item.Description)
		types = append(types, item.Type)
	}
	assert.ElementsMatch(t, webhook.EventTypes(), types)
}

func TestWebhookEndpointAuthorization(t *testing.T) {
	ctx := context.Background()
	productContext := createTestProductContext(t)
	created := createEndpoint(
		t, productContext, "https://receiver.example.com/authz", []string{"license.*"},
	)

	t.Run("UnauthenticatedCallsAreRejected", func(t *testing.T) {
		anonymous := unauthenticatedClient(t)

		listResp, err := anonymous.ListWebhookEndpointsWithResponse(ctx, productContext.ProductID)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, listResp.StatusCode())

		createResp, err := anonymous.CreateWebhookEndpointWithResponse(
			ctx, productContext.ProductID,
			ct.CreateWebhookEndpointJSONRequestBody{
				Url:        receiverURL,
				EventTypes: []string{"license.*"},
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, createResp.StatusCode())

		catalogResp, err := anonymous.ListWebhookEventTypesWithResponse(ctx)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, catalogResp.StatusCode())
	})

	t.Run("AProductApiKeyCannotReachTheAdminSurface", func(t *testing.T) {
		// The webhook surface is platform-admin only; a product API key is a
		// different credential class and must not be accepted here.
		resp, err := productContext.AllScopeAPIKeyClient().ListWebhookEndpointsWithResponse(
			ctx, productContext.ProductID,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
	})

	t.Run("AnotherProductCannotSeeTheEndpoint", func(t *testing.T) {
		otherProduct := createTestProductContext(t)

		// Reading the endpoint under a foreign product must not leak it, even
		// though the caller is a legitimate platform user of that product.
		resp, err := otherProduct.OwnerAuthenticatedClient().GetWebhookEndpointWithResponse(
			ctx, otherProduct.ProductID, created.Endpoint.Id,
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())

		listResp, err := otherProduct.OwnerAuthenticatedClient().
			ListWebhookEndpointsWithResponse(ctx, otherProduct.ProductID)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, listResp.StatusCode())
		require.NotNil(t, listResp.JSON200)
		assert.Empty(t, listResp.JSON200.Items)
	})
}

func TestWebhookDeliveryLog(t *testing.T) {
	ctx := context.Background()
	productContext := createTestProductContext(t)
	client := productContext.OwnerAuthenticatedClient()

	created := createEndpoint(
		t, productContext, "https://receiver.example.com/log",
		[]string{"license.*", webhook.EventTypePing},
	)

	pingResp, pingErr := client.PingWebhookEndpointWithResponse(
		ctx, productContext.ProductID, created.Endpoint.Id,
	)
	require.NoError(t, pingErr)
	require.Equal(t, http.StatusAccepted, pingResp.StatusCode())
	require.NotNil(t, pingResp.JSON202)
	assert.Equal(t, webhook.EventTypePing, pingResp.JSON202.EventType)
	assert.NotEmpty(t, pingResp.JSON202.EventId)

	t.Run("TheLogIsQueryableAndFilterable", func(t *testing.T) {
		// The worker picks the ping up in the background; the log itself must be
		// reachable regardless of whether a delivery has landed yet.
		listResp, err := client.ListWebhookDeliveriesWithResponse(
			ctx, productContext.ProductID, created.Endpoint.Id,
			&ct.ListWebhookDeliveriesParams{},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, listResp.StatusCode())
		require.NotNil(t, listResp.JSON200)

		unmatchedType := "license.created"
		filteredResp, err := client.ListWebhookDeliveriesWithResponse(
			ctx, productContext.ProductID, created.Endpoint.Id,
			&ct.ListWebhookDeliveriesParams{EventType: &unmatchedType},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, filteredResp.StatusCode())
		require.NotNil(t, filteredResp.JSON200)
		assert.Empty(
			t, filteredResp.JSON200.Items,
			"filtering by an event type the endpoint never received returns nothing",
		)
	})

	t.Run("AWellFormedButUnknownDeliveryIs404", func(t *testing.T) {
		resp, err := client.GetWebhookDeliveryWithResponse(
			ctx, productContext.ProductID, created.Endpoint.Id, ids.MustNew("whd"),
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("AMalformedDeliveryIdIs400", func(t *testing.T) {
		// The KSUID pattern on the path parameter rejects the request before it
		// ever reaches the service.
		resp, err := client.GetWebhookDeliveryWithResponse(
			ctx, productContext.ProductID, created.Endpoint.Id, "not-a-ksuid",
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
	})

	t.Run("TheLogOfAnUnknownEndpointIs404", func(t *testing.T) {
		resp, err := client.ListWebhookDeliveriesWithResponse(
			ctx, productContext.ProductID, ids.MustNew("whe"),
			&ct.ListWebhookDeliveriesParams{},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}

// TestWebhookEndpointResponseShape pins the JSON contract the admin UI reads.
func TestWebhookEndpointResponseShape(t *testing.T) {
	productContext := createTestProductContext(t)
	created := createEndpoint(
		t, productContext, "https://receiver.example.com/shape", []string{"license.*"},
	)

	resp, err := productContext.OwnerAuthenticatedClient().GetWebhookEndpointWithResponse(
		context.Background(), productContext.ProductID, created.Endpoint.Id,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode())

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(resp.Body, &decoded))

	for _, field := range []string{
		"id", "product_id", "url", "event_types", "status",
		"consecutive_failure_count", "created_at", "updated_at",
	} {
		assert.Containsf(t, decoded, field, "response must always carry %q", field)
	}
	assert.NotContains(t, decoded, "secret")
}
