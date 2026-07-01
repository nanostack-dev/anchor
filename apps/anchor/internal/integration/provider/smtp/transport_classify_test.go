//nolint:testpackage // verifies unexported envelope-error classification helpers.
package smtp

import (
	"errors"
	"fmt"
	"net/textproto"
	"testing"

	"anchor/internal/integration/provider"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPermanentSMTPRejection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "permanent 5xx reply is a rejection",
			err:  fmt.Errorf("smtp RCPT TO x: %w", &textproto.Error{Code: 553, Msg: "Sender address rejected"}),
			want: true,
		},
		{
			name: "unknown recipient 550 is a rejection",
			err:  &textproto.Error{Code: 550, Msg: "No such user"},
			want: true,
		},
		{
			name: "transient 4xx reply is not a permanent rejection",
			err:  fmt.Errorf("smtp RCPT TO x: %w", &textproto.Error{Code: 451, Msg: "Try again later"}),
			want: false,
		},
		{
			name: "non-protocol transport error is not a rejection",
			err:  errors.New("dial tcp: connection refused"),
			want: false,
		},
		{
			name: "nil error is not a rejection",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isPermanentSMTPRejection(tc.err))
		})
	}
}

func TestClassifyEnvelopeErr(t *testing.T) {
	t.Parallel()

	t.Run("permanent rejection is tagged with ErrMessageRejected", func(t *testing.T) {
		t.Parallel()
		cause := &textproto.Error{Code: 553, Msg: "Sender address rejected"}
		wrapped := fmt.Errorf("smtp RCPT TO bob@example.com: %w", cause)

		got := classifyEnvelopeErr(wrapped, cause)

		require.ErrorIs(t, got, provider.ErrMessageRejected,
			"permanent 5xx rejection must carry provider.ErrMessageRejected")
		// The command/address context is preserved for server-side logs.
		assert.Contains(t, got.Error(), "smtp RCPT TO bob@example.com")
	})

	t.Run("transient failure propagates unchanged", func(t *testing.T) {
		t.Parallel()
		cause := &textproto.Error{Code: 451, Msg: "Try again later"}
		wrapped := fmt.Errorf("smtp RCPT TO bob@example.com: %w", cause)

		got := classifyEnvelopeErr(wrapped, cause)

		require.NotErrorIs(t, got, provider.ErrMessageRejected,
			"transient 4xx failure must not be classified as a permanent rejection")
		assert.Equal(t, wrapped, got)
	})
}
