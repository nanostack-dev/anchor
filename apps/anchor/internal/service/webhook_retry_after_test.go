package service_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"anchor/internal/service"
)

// TestRetryAfterError covers the round trip a receiver-requested delay makes:
// a delivery attempt wraps it into the error it returns, and the worker's
// RetryDelay func has to recover it from whatever pgqueue hands back.
func TestRetryAfterError(t *testing.T) {
	t.Run("nil delay leaves the error untouched", func(t *testing.T) {
		base := errors.New("attempt failed")

		got := service.NewRetryAfterError(nil, base)

		assert.Equal(t, base, got)
		_, ok := service.RetryAfterDelay(got)
		assert.False(t, ok, "an error without a delay must fall back to the ladder")
	})

	t.Run("delay survives the round trip", func(t *testing.T) {
		delay := 90 * time.Second

		got := service.NewRetryAfterError(&delay, errors.New("429 too many requests"))

		recovered, ok := service.RetryAfterDelay(got)
		assert.True(t, ok)
		assert.Equal(t, delay, recovered)
		assert.Equal(t, "429 too many requests", got.Error())
	})

	t.Run("delay is recovered through a wrapping error", func(t *testing.T) {
		delay := 5 * time.Minute
		wrapped := fmt.Errorf("deliver webhook: %w",
			service.NewRetryAfterError(&delay, errors.New("503 unavailable")))

		recovered, ok := service.RetryAfterDelay(wrapped)
		assert.True(t, ok)
		assert.Equal(t, delay, recovered)
	})

	t.Run("a plain error carries no delay", func(t *testing.T) {
		_, ok := service.RetryAfterDelay(errors.New("connection reset"))
		assert.False(t, ok)
	})
}
