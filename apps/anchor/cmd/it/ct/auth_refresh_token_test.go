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
	assert.NotEmpty(t, testOwnerUser(t).RefreshToken)

	t.Run(
		"ValidRefreshToken", func(t *testing.T) {
			refreshParams := &ct.RefreshTokenParams{
				RefreshToken: &testOwnerUser(t).RefreshToken,
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
		"MissingRefreshToken", func(t *testing.T) {
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
		"InvalidRefreshToken", func(t *testing.T) {
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
