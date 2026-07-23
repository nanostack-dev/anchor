package webhook_test

import (
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

func TestEventTypeCatalogIsACopy(t *testing.T) {
	t.Parallel()

	catalog := webhook.EventTypeCatalog()
	require.NotEmpty(t, catalog)

	catalog[0].Type = "mutated"
	assert.NotEqual(t, "mutated", webhook.EventTypeCatalog()[0].Type)
}
