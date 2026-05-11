package ct_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCurrentUser(t *testing.T) {
	assert.NotEmpty(t, testOwnerUser(t).AccessToken)

	t.Run(
		"ValidAuthentication", func(t *testing.T) {
			resp, err := testOwnerClient(t).GetCurrentUserWithResponse(
				t.Context(),
			)
			require.NoError(t, err, "get current productuserservice request should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)
			assert.Equal(t, testOwnerUser(t).Email, resp.JSON200.Email)
		},
	)
}
