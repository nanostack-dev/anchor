package ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogout(t *testing.T) {
	assert.NotEmpty(t, testOwnerUser(t).AccessToken)

	t.Run(
		"ValidAuthentication", func(t *testing.T) {
			resp, err := testOwnerClient(t).LogoutWithResponse(
				t.Context(),
			)
			require.NoError(t, err, "logout request should not error")
			assert.Equal(t, http.StatusNoContent, resp.StatusCode())
		},
	)
}
