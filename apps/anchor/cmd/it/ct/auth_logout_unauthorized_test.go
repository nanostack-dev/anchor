package ct_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogoutWithoutAuthentication(t *testing.T) {
	t.Run(
		"UnauthenticatedLogout", func(t *testing.T) {
			// Test that logout works even without authentication headers
			resp, err := testTenant(t).NoAuthClient.LogoutWithResponse(
				context.Background(),
			)
			require.NoError(t, err, "logout request should not error even without auth")
			assert.Equal(
				t, http.StatusNoContent, resp.StatusCode(),
				"logout should succeed without authentication",
			)
		},
	)
}
