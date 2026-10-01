package ct_test

import (
	"net/http"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMissingCredentialReturnsJSONErrorEnvelope(t *testing.T) {
	t.Parallel()

	resp, err := testTenant(t).NoAuthClient.GetCurrentUserWithResponse(t.Context())
	require.NoError(t, err)

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
	assert.Contains(t, resp.HTTPResponse.Header.Get("Content-Type"), "application/json")
	require.NotNil(t, resp.JSON401, string(resp.Body))
	require.NotEmpty(t, resp.JSON401.Errors)
	assert.Equal(t, "UNAUTHORIZED", resp.JSON401.Errors[0].Code)
}

func TestAuthUnknownProductReturnsJSONErrorEnvelope(t *testing.T) {
	t.Parallel()

	resp, err := testOwnerClient(t).GetProductWithResponse(t.Context(), ids.MustNew("prd"))
	require.NoError(t, err)

	require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	assert.Contains(t, resp.HTTPResponse.Header.Get("Content-Type"), "application/json")
	require.NotNil(t, resp.JSON404, string(resp.Body))
	require.NotEmpty(t, resp.JSON404.Errors)
	assert.Equal(t, "PRODUCT_NOT_FOUND", resp.JSON404.Errors[0].Code)
}
