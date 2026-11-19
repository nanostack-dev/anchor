package events_test

import (
	"testing"
	"time"

	"anchor/internal/events"
	serviceconfig "anchor/internal/service/config"

	"github.com/stretchr/testify/assert"
)

func TestDeliveryRetryPolicy(t *testing.T) {
	t.Parallel()

	for _, environment := range []string{"prod", "production", "dev", "development"} {
		t.Run("A deployment retries a failing endpoint for at least 12 hours in "+environment, func(t *testing.T) {
			t.Parallel()
			core := &serviceconfig.CoreConfig{Environment: environment}
			attempts, shortest, _ := events.DeliveryRetryWindowForTest(core)
			assert.Equal(t, 20, attempts)
			assert.GreaterOrEqual(t, shortest, 12*time.Hour)
		})
	}

	t.Run("The test suites give up on a failing endpoint within a second after six attempts", func(t *testing.T) {
		t.Parallel()
		attempts, _, longest := events.DeliveryRetryWindowForTest(&serviceconfig.CoreConfig{Environment: "test"})
		assert.Equal(t, 6, attempts)
		assert.Less(t, longest, time.Second)
	})
}

func TestDeliveryRetryDelay(t *testing.T) {
	t.Parallel()

	t.Run("Each delay keeps at least three quarters of its doubled base and never passes the cap", func(t *testing.T) {
		t.Parallel()
		prod := &serviceconfig.CoreConfig{Environment: "prod"}
		for jobID := int64(1); jobID <= 50; jobID++ {
			first := events.DeliveryRetryDelayForTest(prod, jobID, 1)
			assert.GreaterOrEqual(t, first, 7500*time.Millisecond)
			assert.LessOrEqual(t, first, 10*time.Second)
			capped := events.DeliveryRetryDelayForTest(prod, jobID, 19)
			assert.GreaterOrEqual(t, capped, 90*time.Minute)
			assert.LessOrEqual(t, capped, 2*time.Hour)
		}
	})
}
