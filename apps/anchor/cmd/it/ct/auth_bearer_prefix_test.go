package ct_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddlewareRequiresBearerPrefix(t *testing.T) {
	t.Parallel()

	token := testOwnerUser(t).AccessToken
	require.NotEmpty(t, token)

	noAuth := testTenant(t).NoAuthClient

	for _, tc := range []struct {
		name   string
		header string
	}{
		{name: "Basic scheme with two spaces", header: "Basic  " + token},
		{name: "Arbitrary 7-byte prefix", header: "XXXXXXX" + token},
		{name: "Lowercase bearer prefix", header: "bearer " + token},
		{name: "Empty token after Bearer prefix", header: "Bearer "},
	} {
		t.Run("Rejects "+tc.name, func(t *testing.T) {
			t.Parallel()

			resp, err := noAuth.GetCurrentUserWithResponse(
				t.Context(),
				func(_ context.Context, req *http.Request) error {
					req.Header.Set("Authorization", tc.header)
					return nil
				},
			)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
		})
	}

	t.Run("Accepts exact Bearer prefix with valid access token", func(t *testing.T) {
		t.Parallel()

		resp, err := noAuth.GetCurrentUserWithResponse(
			t.Context(),
			func(_ context.Context, req *http.Request) error {
				req.Header.Set("Authorization", "Bearer "+token)
				return nil
			},
		)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	})
}
