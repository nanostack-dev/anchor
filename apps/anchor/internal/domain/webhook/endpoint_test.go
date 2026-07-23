package webhook_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/webhook"
)

func TestShouldAutoDisableRequiresBothConditions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	old := now.Add(-25 * time.Hour)
	recent := now.Add(-10 * time.Minute)

	tests := []struct {
		name     string
		counters webhook.FailureCounters
		want     bool
	}{
		{
			name: "both conditions hold",
			counters: webhook.FailureCounters{
				ConsecutiveFailureCount: webhook.AutoDisableFailureThreshold,
				FirstFailureAt:          &old,
			},
			want: true,
		},
		{
			name: "enough failures but the streak is young — a deploy blip",
			counters: webhook.FailureCounters{
				ConsecutiveFailureCount: 50,
				FirstFailureAt:          &recent,
			},
			want: false,
		},
		{
			name: "old streak but too few failures — merely a quiet endpoint",
			counters: webhook.FailureCounters{
				ConsecutiveFailureCount: webhook.AutoDisableFailureThreshold - 1,
				FirstFailureAt:          &old,
			},
			want: false,
		},
		{
			name: "no streak at all",
			counters: webhook.FailureCounters{
				ConsecutiveFailureCount: 0,
			},
			want: false,
		},
		{
			name: "failure count without a first-failure stamp cannot be aged",
			counters: webhook.FailureCounters{
				ConsecutiveFailureCount: 100,
			},
			want: false,
		},
		{
			name: "exactly at both boundaries",
			counters: webhook.FailureCounters{
				ConsecutiveFailureCount: webhook.AutoDisableFailureThreshold,
				FirstFailureAt:          new(now.Add(-webhook.AutoDisableMinFailureAge)),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, webhook.ShouldAutoDisable(tt.counters, now))
		})
	}
}

func TestRecordFailureStampsTheStreakStart(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)

	counters := webhook.RecordFailure(webhook.FailureCounters{}, first)
	require.NotNil(t, counters.FirstFailureAt)
	assert.Equal(t, int32(1), counters.ConsecutiveFailureCount)
	assert.Equal(t, first, *counters.FirstFailureAt)

	counters = webhook.RecordFailure(counters, second)
	assert.Equal(t, int32(2), counters.ConsecutiveFailureCount)
	assert.Equal(t, first, *counters.FirstFailureAt, "the streak start must not move")
	require.NotNil(t, counters.LastFailureAt)
	assert.Equal(t, second, *counters.LastFailureAt)
}

func TestRecordSuccessClearsBothHalvesOfTheRule(t *testing.T) {
	t.Parallel()

	failedAt := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	succeededAt := failedAt.Add(48 * time.Hour)

	counters := webhook.FailureCounters{
		ConsecutiveFailureCount: 100,
		FirstFailureAt:          &failedAt,
		LastFailureAt:           &failedAt,
	}

	counters = webhook.RecordSuccess(counters, succeededAt)
	assert.Equal(t, int32(0), counters.ConsecutiveFailureCount)
	assert.Nil(t, counters.FirstFailureAt)
	require.NotNil(t, counters.LastSuccessAt)
	assert.Equal(t, succeededAt, *counters.LastSuccessAt)
	assert.False(t, webhook.ShouldAutoDisable(counters, succeededAt))
}

func TestRecordFailureAfterSuccessRestartsTheStreak(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

	counters := webhook.RecordSuccess(webhook.FailureCounters{}, start)
	counters = webhook.RecordFailure(counters, start.Add(time.Hour))

	require.NotNil(t, counters.FirstFailureAt)
	assert.Equal(t, start.Add(time.Hour), *counters.FirstFailureAt)
	assert.Equal(t, int32(1), counters.ConsecutiveFailureCount)
}

func TestEndpointSubscribes(t *testing.T) {
	t.Parallel()

	endpoint := &webhook.Endpoint{EventTypes: []string{"license.*"}}
	assert.True(t, endpoint.Subscribes(webhook.EventTypeLicenseRevoked))
	assert.False(t, endpoint.Subscribes(webhook.EventTypePlanUpdated))
}

func TestEndpointStatusValidity(t *testing.T) {
	t.Parallel()

	assert.True(t, webhook.EndpointStatusEnabled.IsValid())
	assert.True(t, webhook.EndpointStatusDisabled.IsValid())
	assert.True(t, webhook.EndpointStatusAutoDisabled.IsValid())
	assert.False(t, webhook.EndpointStatus("PAUSED").IsValid())

	enabled := webhook.Endpoint{Status: webhook.EndpointStatusEnabled}
	autoDisabled := webhook.Endpoint{Status: webhook.EndpointStatusAutoDisabled}
	assert.True(t, enabled.IsEnabled())
	assert.False(t, autoDisabled.IsEnabled())
}
