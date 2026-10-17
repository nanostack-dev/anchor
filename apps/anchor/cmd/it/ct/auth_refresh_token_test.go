package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshToken(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, testOwnerUser(t).RefreshToken)

	t.Run(
		"A valid refresh token returns a new access token", func(t *testing.T) {
			refreshParams := &ct.RefreshTokenParams{
				RefreshToken: &createPlatformAdmin(t).RefreshToken,
			}
			refreshResp, err := testTenant(t).NoAuthClient.RefreshTokenWithResponse(
				context.Background(), refreshParams,
			)
			require.NoError(t, err, "refresh token request should not error")
			assert.Equal(t, http.StatusOK, refreshResp.StatusCode())
			assert.NotNil(t, refreshResp.JSON200)
			assert.NotEmpty(t, refreshResp.JSON200.AccessToken)
		},
	)

	t.Run(
		"Refreshing without a refresh token is refused", func(t *testing.T) {
			refreshParams := &ct.RefreshTokenParams{
				RefreshToken: nil,
			}
			resp, err := testTenant(t).NoAuthClient.RefreshTokenWithResponse(
				context.Background(), refreshParams,
			)
			require.NoError(t, err, "refresh token request should not error")
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
		},
	)

	t.Run(
		"Refreshing with an invalid token is refused", func(t *testing.T) {
			invalidToken := "invalid-refresh-token-123"
			refreshParams := &ct.RefreshTokenParams{
				RefreshToken: &invalidToken,
			}
			resp, err := testTenant(t).NoAuthClient.RefreshTokenWithResponse(
				context.Background(), refreshParams,
			)
			require.NoError(t, err, "refresh token request should not error")
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
		},
	)
}
