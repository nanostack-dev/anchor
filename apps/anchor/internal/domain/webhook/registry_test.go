package webhook_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/webhook"
)

func TestMatchesSubscription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		subscribed []string
		eventType  string
		want       bool
	}{
		{
			name:       "exact match",
			subscribed: []string{webhook.EventTypeLicenseRevoked},
			eventType:  webhook.EventTypeLicenseRevoked,
			want:       true,
		},
		{
			name:       "exact mismatch",
			subscribed: []string{webhook.EventTypeLicenseCreated},
			eventType:  webhook.EventTypeLicenseRevoked,
			want:       false,
		},
		{
			name:       "group wildcard matches every event of the group",
			subscribed: []string{"license.*"},
			eventType:  webhook.EventTypeLicenseUpdated,
			want:       true,
		},
		{
			name:       "group wildcard does not cross groups",
			subscribed: []string{"license.*"},
			eventType:  webhook.EventTypePlanUpdated,
			want:       false,
		},
		{
			name:       "group wildcard does not match the bare group name",
			subscribed: []string{"license.*"},
			eventType:  "license",
			want:       false,
		},
		{
			name:       "group wildcard does not match a longer group prefix",
			subscribed: []string{"license.*"},
			eventType:  "licenses.created",
			want:       false,
		},
		{
			name:       "wildcard matches nested segments of its group",
			subscribed: []string{"license.*"},
			eventType:  "license.seat.added",
			want:       true,
		},
		{
			name:       "any entry may match",
			subscribed: []string{webhook.EventTypePing, "license.*"},
			eventType:  webhook.EventTypeLicenseCreated,
			want:       true,
		},
		{
			name:       "empty subscription list matches nothing",
			subscribed: nil,
			eventType:  webhook.EventTypeLicenseCreated,
			want:       false,
		},
		{
			name:       "a bare star is not a supported wildcard",
			subscribed: []string{"*"},
			eventType:  webhook.EventTypeLicenseCreated,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t, tt.want, webhook.MatchesSubscription(tt.subscribed, tt.eventType),
			)
		})
	}
}

func TestValidateEventType(t *testing.T) {
	t.Parallel()

	for _, eventType := range webhook.EventTypes() {
		require.NoErrorf(t, webhook.Validate(eventType), "registered type %q must validate", eventType)
	}

	require.Error(t, webhook.Validate("license.exploded"))
	require.Error(t, webhook.Validate(""))
	require.Error(t, webhook.Validate("license.*"))
}

func TestValidateSubscription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		subscription string
		wantErr      bool
	}{
		{name: "exact registered type", subscription: webhook.EventTypeLicenseCreated},
		{name: "registered group wildcard", subscription: "license.*"},
		{name: "plan group wildcard", subscription: "plan.*"},
		{name: "unknown type", subscription: "billing.charged", wantErr: true},
		{name: "unknown group wildcard", subscription: "billing.*", wantErr: true},
		{name: "empty", subscription: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := webhook.ValidateSubscription(tt.subscription)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestEveryEventTypeHasAValidSamplePayload is what keeps the test-event surface
// honest. A registry entry added without a sample is a type the admin UI offers
// but cannot send, and a sample that does not marshal is a 500 on the catalog
// endpoint; both fail here rather than in front of a user.
func TestEveryEventTypeHasAValidSamplePayload(t *testing.T) {
	t.Parallel()

	catalog := webhook.EventTypeCatalog()
	require.NotEmpty(t, catalog)

	for _, descriptor := range catalog {
		t.Run(descriptor.Type, func(t *testing.T) {
			t.Parallel()

			require.NotNilf(
				t, descriptor.Sample,
				"registered type %q must carry a sample payload", descriptor.Type,
			)

			encoded, err := webhook.SamplePayloadJSON(descriptor.Type)
			require.NoError(t, err)
			require.NotEmpty(t, encoded)

			var decoded map[string]any
			require.NoErrorf(
				t, json.Unmarshal([]byte(encoded), &decoded),
				"the sample for %q must be a JSON object, which is what `data` is",
				descriptor.Type,
			)
			assert.NotEmptyf(
				t, decoded,
				"an empty sample for %q teaches a receiver nothing", descriptor.Type,
			)
		})
	}
}

func TestSamplePayloadRejectsUnknownTypes(t *testing.T) {
	t.Parallel()

	_, err := webhook.SamplePayload("license.exploded")
	require.Error(t, err)

	_, err = webhook.SamplePayload("license.*")
	require.Error(t, err, "a wildcard is a subscription, never a sendable type")
}

func TestTestEventDataNamesTheProbedEndpoint(t *testing.T) {
	t.Parallel()

	const endpointID = "whe_2t7Yc4nQwLpZbXmVkR9sHdF3jA1"

	data, err := webhook.TestEventData(webhook.EventTypePing, endpointID)
	require.NoError(t, err)

	probe, ok := data.(webhook.PingEventData)
	require.True(t, ok)
	assert.Equal(t, endpointID, probe.EndpointID)
	assert.Equal(t, webhook.PingMessage, probe.Message)

	// The published sample keeps its placeholder; only the send is rewritten.
	sample, err := webhook.SamplePayload(webhook.EventTypePing)
	require.NoError(t, err)
	assert.Equal(t, webhook.SampleEndpointID, sample.(webhook.PingEventData).EndpointID)
}

func TestTestEventDataPassesOtherSamplesThrough(t *testing.T) {
	t.Parallel()

	data, err := webhook.TestEventData(webhook.EventTypeLicenseRevoked, "whe_ignored")
	require.NoError(t, err)

	license, ok := data.(webhook.LicenseEventData)
	require.True(t, ok)
	assert.Equal(t, webhook.SampleLicenseID, license.LicenseID)
}

func TestEventTypeCatalogIsACopy(t *testing.T) {
	t.Parallel()

	catalog := webhook.EventTypeCatalog()
	require.NotEmpty(t, catalog)

	catalog[0].Type = "mutated"
	assert.NotEqual(t, "mutated", webhook.EventTypeCatalog()[0].Type)
}
