package webhook_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/webhook"
)

func TestEnvelopeShape(t *testing.T) {
	t.Parallel()

	organizationID := "org_2iABC"
	event := webhook.Event{
		ID:             "evt_2gH",
		ProductID:      "prod_2iABC",
		OrganizationID: &organizationID,
		EventType:      webhook.EventTypeLicenseUpdated,
		APIVersion:     webhook.APIVersion,
		Payload: json.RawMessage(
			`{"license_id":"lic_1","plan_key":"pro","status":"SUSPENDED"}`,
		),
		OccurredAt: time.Date(2026, 7, 23, 14, 2, 11, 0, time.UTC),
	}

	encoded, err := json.Marshal(event.Envelope())
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	assert.Equal(t, "evt_2gH", decoded["id"])
	assert.Equal(t, webhook.EventTypeLicenseUpdated, decoded["type"])
	assert.Equal(t, webhook.APIVersion, decoded["api_version"])
	assert.Equal(t, "2026-07-23T14:02:11Z", decoded["occurred_at"])
	assert.Equal(t, "prod_2iABC", decoded["product_id"])
	assert.Equal(t, "org_2iABC", decoded["organization_id"])
	assert.Equal(
		t, false, decoded["test"],
		"a real event says so explicitly; an absent field would be ambiguous",
	)

	data, ok := decoded["data"].(map[string]any)
	require.True(t, ok, "data must be a nested object, not a string")
	assert.Equal(t, "lic_1", data["license_id"])
	assert.Equal(t, "SUSPENDED", data["status"])
}

func TestEnvelopeOmitsOrganizationForProductScopedEvents(t *testing.T) {
	t.Parallel()

	event := webhook.Event{
		ID:         "evt_plan",
		ProductID:  "prod_2iABC",
		EventType:  webhook.EventTypePlanUpdated,
		APIVersion: webhook.APIVersion,
		OccurredAt: time.Now(),
	}

	encoded, err := json.Marshal(event.Envelope())
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "organization_id")
	assert.Contains(t, string(encoded), `"data":{}`, "an empty payload still renders an object")
}

func TestEnvelopeMarksTargetedSendsAsTests(t *testing.T) {
	t.Parallel()

	endpointID := "whe_2t7Yc4nQwLpZbXmVkR9sHdF3jA1"
	event := webhook.Event{
		ID:               "evt_test",
		ProductID:        "prod_2iABC",
		EventType:        webhook.EventTypePing,
		APIVersion:       webhook.APIVersion,
		OccurredAt:       time.Now(),
		TargetEndpointID: &endpointID,
	}

	assert.True(t, event.IsTest())

	encoded, err := json.Marshal(event.Envelope())
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"test":true`)

	broadcast := event
	broadcast.TargetEndpointID = nil
	assert.False(t, broadcast.IsTest())
}

func TestTruncateSnippetAndError(t *testing.T) {
	t.Parallel()

	short := []byte("ok")
	assert.Equal(t, "ok", webhook.TruncateSnippet(short))

	// A hostile receiver could otherwise echo unbounded content into the admin UI.
	long := []byte(strings.Repeat("x", webhook.MaxResponseSnippetBytes*4))
	assert.Len(t, webhook.TruncateSnippet(long), webhook.MaxResponseSnippetBytes)

	assert.Equal(t, "boom", webhook.TruncateError("boom"))
	assert.Len(
		t,
		webhook.TruncateError(strings.Repeat("e", webhook.MaxErrorLength*2)),
		webhook.MaxErrorLength,
	)
}

func TestDeliveryStatusHelpers(t *testing.T) {
	t.Parallel()

	assert.True(t, webhook.DeliveryStatusPending.IsValid())
	assert.False(t, webhook.DeliveryStatus("SENDING").IsValid())

	assert.False(t, webhook.DeliveryStatusPending.IsTerminal())
	assert.True(t, webhook.DeliveryStatusSucceeded.IsTerminal())
	assert.True(t, webhook.DeliveryStatusFailed.IsTerminal())
	assert.True(t, webhook.DeliveryStatusExhausted.IsTerminal())

	delivery := webhook.Delivery{AttemptCount: 7, MaxAttempts: webhook.MaxDeliveryAttempts}
	assert.True(t, delivery.AttemptsRemaining())
	delivery.AttemptCount = webhook.MaxDeliveryAttempts
	assert.False(t, delivery.AttemptsRemaining())

	original := webhook.Delivery{}
	replay := webhook.Delivery{ReplayOfDeliveryID: new("whd_original")}
	assert.False(t, original.IsReplay())
	assert.True(t, replay.IsReplay())
}
