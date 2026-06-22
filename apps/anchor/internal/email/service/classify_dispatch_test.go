//nolint:testpackage // verifies the unexported dispatch-error classifier.
package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/textproto"
	"testing"

	"anchor/internal/integration/provider"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyDispatchError(t *testing.T) {
	t.Parallel()

	t.Run("permanent relay rejection maps to 422 EMAIL_DELIVERY_REJECTED", func(t *testing.T) {
		t.Parallel()
		// Shape mirrors what the SMTP transport returns for a 553 sender / 550
		// recipient rejection: the cause wrapped with provider.ErrMessageRejected.
		cause := fmt.Errorf("smtp RCPT TO bob@example.com: %w",
			&textproto.Error{Code: 553, Msg: "Sender address rejected"})
		dispatchErr := fmt.Errorf("%w: %w", provider.ErrMessageRejected, cause)

		got := classifyDispatchError(dispatchErr)

		apiErr, ok := fault.As(got)
		require.True(t, ok, "result must be a modelled fault")
		assert.Equal(t, http.StatusUnprocessableEntity, apiErr.HTTPStatus())
		assert.Equal(t, "EMAIL_DELIVERY_REJECTED", apiErr.Details[0].Code)
		// The transport cause stays reachable for server-side logs.
		assert.ErrorIs(t, got, provider.ErrMessageRejected)
		// ...but is never exposed in the client-safe message.
		assert.NotContains(t, apiErr.Message(), "Sender address rejected")
	})

	t.Run("transport failure maps to 500 EMAIL_DELIVERY_FAILED", func(t *testing.T) {
		t.Parallel()
		dispatchErr := errors.New("dial tcp 127.0.0.1:2525: connect: connection refused")

		got := classifyDispatchError(dispatchErr)

		apiErr, ok := fault.As(got)
		require.True(t, ok, "result must be a modelled fault")
		assert.Equal(t, http.StatusInternalServerError, apiErr.HTTPStatus())
		assert.Equal(t, "EMAIL_DELIVERY_FAILED", apiErr.Details[0].Code)
		assert.NotContains(t, apiErr.Message(), "connection refused")
	})

	t.Run("context cancellation is returned unchanged", func(t *testing.T) {
		t.Parallel()
		got := classifyDispatchError(context.Canceled)

		assert.ErrorIs(t, got, context.Canceled)
		_, ok := fault.As(got)
		assert.False(t, ok, "context cancellation must not be wrapped in a fault")
	})
}
