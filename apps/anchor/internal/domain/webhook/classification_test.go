package webhook_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/webhook"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		statusCode   int
		transportErr error
		want         webhook.Outcome
	}{
		{name: "200 succeeds", statusCode: http.StatusOK, want: webhook.OutcomeSucceeded},
		{name: "201 succeeds", statusCode: http.StatusCreated, want: webhook.OutcomeSucceeded},
		{name: "204 succeeds", statusCode: http.StatusNoContent, want: webhook.OutcomeSucceeded},
		{name: "299 succeeds", statusCode: 299, want: webhook.OutcomeSucceeded},
		{
			name:       "301 is a permanent misconfiguration because redirects are refused",
			statusCode: http.StatusMovedPermanently, want: webhook.OutcomeFailed,
		},
		{
			name:       "302 is a permanent misconfiguration because redirects are refused",
			statusCode: http.StatusFound, want: webhook.OutcomeFailed,
		},
		{name: "400 fails permanently", statusCode: http.StatusBadRequest, want: webhook.OutcomeFailed},
		{name: "401 fails permanently", statusCode: http.StatusUnauthorized, want: webhook.OutcomeFailed},
		{name: "404 fails permanently", statusCode: http.StatusNotFound, want: webhook.OutcomeFailed},
		{name: "408 retries", statusCode: http.StatusRequestTimeout, want: webhook.OutcomeRetry},
		{
			name:       "410 disables the endpoint",
			statusCode: http.StatusGone, want: webhook.OutcomeDisableEndpoint,
		},
		{name: "422 fails permanently", statusCode: http.StatusUnprocessableEntity, want: webhook.OutcomeFailed},
		{name: "429 retries", statusCode: http.StatusTooManyRequests, want: webhook.OutcomeRetry},
		{name: "500 retries", statusCode: http.StatusInternalServerError, want: webhook.OutcomeRetry},
		{name: "502 retries", statusCode: http.StatusBadGateway, want: webhook.OutcomeRetry},
		{name: "503 retries", statusCode: http.StatusServiceUnavailable, want: webhook.OutcomeRetry},
		{
			name:         "a transport error always retries",
			statusCode:   0,
			transportErr: errors.New("dial tcp: i/o timeout"),
			want:         webhook.OutcomeRetry,
		},
		{
			name:         "a transport error wins over a status code",
			statusCode:   http.StatusNotFound,
			transportErr: errors.New("connection reset"),
			want:         webhook.OutcomeRetry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, webhook.Classify(tt.statusCode, tt.transportErr))
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

	t.Run("absent header yields no delay", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, webhook.ParseRetryAfter("", now))
	})

	t.Run("unparseable header yields no delay", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, webhook.ParseRetryAfter("soon", now))
	})

	t.Run("delta seconds", func(t *testing.T) {
		t.Parallel()
		delay := webhook.ParseRetryAfter("120", now)
		require.NotNil(t, delay)
		assert.Equal(t, 2*time.Minute, *delay)
	})

	t.Run("http date", func(t *testing.T) {
		t.Parallel()
		delay := webhook.ParseRetryAfter(now.Add(time.Hour).Format(http.TimeFormat), now)
		require.NotNil(t, delay)
		assert.Equal(t, time.Hour, *delay)
	})

	t.Run("a hostile value is capped", func(t *testing.T) {
		t.Parallel()
		delay := webhook.ParseRetryAfter("99999999", now)
		require.NotNil(t, delay)
		assert.Equal(t, webhook.MaxRetryAfter, *delay)
	})

	t.Run("a past date floors at zero", func(t *testing.T) {
		t.Parallel()
		delay := webhook.ParseRetryAfter(now.Add(-time.Hour).Format(http.TimeFormat), now)
		require.NotNil(t, delay)
		assert.Equal(t, time.Duration(0), *delay)
	})
}
