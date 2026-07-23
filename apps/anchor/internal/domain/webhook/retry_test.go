package webhook_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/webhook"
)

// fixedJitterer pins the jitter sample so a test can assert exact delays.
type fixedJitterer struct {
	sample float64
}

func (f fixedJitterer) Float64() float64 {
	return f.sample
}

func TestRetryDelayLadder(t *testing.T) {
	t.Parallel()

	// A 0.5 sample lands on the centre of the [0.75, 1.25] window: 1.0x.
	centre := fixedJitterer{sample: 0.5}

	tests := []struct {
		name        string
		attemptsMad int
		want        time.Duration
	}{
		{name: "never attempted fires immediately", attemptsMad: 0, want: 0},
		{name: "after 1 attempt", attemptsMad: 1, want: 15 * time.Second},
		{name: "after 2 attempts", attemptsMad: 2, want: 1 * time.Minute},
		{name: "after 3 attempts", attemptsMad: 3, want: 5 * time.Minute},
		{name: "after 4 attempts", attemptsMad: 4, want: 30 * time.Minute},
		{name: "after 5 attempts", attemptsMad: 5, want: 2 * time.Hour},
		{name: "after 6 attempts", attemptsMad: 6, want: 6 * time.Hour},
		{name: "after 7 attempts", attemptsMad: 7, want: 12 * time.Hour},
		{name: "past the ladder stays on the last rung", attemptsMad: 20, want: 12 * time.Hour},
		{name: "a negative count is treated as zero", attemptsMad: -3, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, webhook.RetryDelay(tt.attemptsMad, centre))
		})
	}
}

func TestRetryDelayJitterStaysInBounds(t *testing.T) {
	t.Parallel()

	// The ladder spans roughly 21 hours; jitter must never move a rung outside
	// [0.75, 1.25] of its base, or the schedule stops being predictable.
	ladder := webhook.RetryLadder()
	for attempt := 1; attempt < len(ladder); attempt++ {
		base := ladder[attempt]
		low := time.Duration(float64(base) * webhook.JitterFloor)
		high := time.Duration(float64(base) * webhook.JitterCeiling)

		for _, sample := range []float64{0, 0.25, 0.5, 0.75, 0.999999} {
			delay := webhook.RetryDelay(attempt, fixedJitterer{sample: sample})
			assert.GreaterOrEqual(t, delay, low)
			assert.LessOrEqual(t, delay, high)
		}
	}
}

func TestRetryDelayImmediateRungIgnoresJitter(t *testing.T) {
	t.Parallel()

	// Multiplying zero by jitter is still zero, but asserting it keeps the
	// "first attempt is immediate" promise from silently regressing.
	assert.Equal(t, time.Duration(0), webhook.RetryDelay(0, fixedJitterer{sample: 0.999}))
}

func TestRetryLadderTotalMatchesTheDocumentedWindow(t *testing.T) {
	t.Parallel()

	ladder := webhook.RetryLadder()
	assert.Len(t, ladder, int(webhook.MaxDeliveryAttempts))

	var total time.Duration
	for _, rung := range ladder {
		total += rung
	}
	assert.Equal(t, 20*time.Hour+36*time.Minute+15*time.Second, total)
}

func TestDefaultJitterer(t *testing.T) {
	t.Parallel()

	jitterer := webhook.DefaultJitterer()
	for range 100 {
		sample := jitterer.Float64()
		assert.GreaterOrEqual(t, sample, 0.0)
		assert.Less(t, sample, 1.0)
	}
}
