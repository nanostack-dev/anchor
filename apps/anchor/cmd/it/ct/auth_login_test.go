package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeAPIError unmarshals a response body into the shared error contract.
// Login's generated client only types the 200 and 400 bodies today; a 401
// still arrives as raw bytes on resp.Body, so tests that assert on the
// EMAIL_OR_PASSWORD_INCORRECT code and message decode it directly instead of
// through a generated JSON401 field.
func decodeAPIError(t *testing.T, body []byte) ct.ApiErrorResponse {
	t.Helper()
	var errResp ct.ApiErrorResponse
	require.NoError(t, json.Unmarshal(body, &errResp), "error body should decode")
	require.NotEmpty(t, errResp.Errors, "error body should carry at least one error")
	return errResp
}

func TestLogin(t *testing.T) {
	assert.NotEmpty(t, testOwnerUser(t).Email)
	assert.NotEmpty(t, testOwnerUser(t).Password)

	t.Run(
		"ValidCredentials", func(t *testing.T) {
			loginReq := ct.LoginJSONRequestBody{
				Email:    testOwnerUser(t).Email,
				Password: testOwnerUser(t).Password,
			}
			resp, err := testTenant(t).NoAuthClient.LoginWithResponse(
				context.Background(), loginReq,
			)
			require.NoError(t, err, "login request should not error")
			assert.Equal(t, http.StatusOK, resp.StatusCode())
			assert.NotNil(t, resp.JSON200)
			assert.NotEmpty(t, resp.JSON200.AccessToken)
			assert.NotEmpty(t, resp.JSON200.RefreshToken)
		},
	)

	t.Run(
		"InvalidPassword", func(t *testing.T) {
			loginReq := ct.LoginJSONRequestBody{
				Email:    testOwnerUser(t).Email,
				Password: "WrongPassword!",
			}
			resp, err := testTenant(t).NoAuthClient.LoginWithResponse(
				context.Background(), loginReq,
			)
			require.NoError(t, err, "login request should not error")
			// A password that does not authenticate is a 401: the request
			// carried a credential, and it was wrong.
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
			errResp := decodeAPIError(t, resp.Body)
			assert.Equal(t, "EMAIL_OR_PASSWORD_INCORRECT", errResp.Errors[0].Code)
			assert.Equal(t, "The email or the password is incorrect.", errResp.Errors[0].Message)
		},
	)

	t.Run(
		"NonexistentUser", func(t *testing.T) {
			loginReq := ct.LoginJSONRequestBody{
				Email:    "doesnotexist@example.com",
				Password: "AnyPassword123!",
			}
			resp, err := testTenant(t).NoAuthClient.LoginWithResponse(
				context.Background(), loginReq,
			)
			require.NoError(t, err, "login request should not error")
			// An email that matches no user is the same credential failure as
			// a wrong password: a 401, not a 404, so a caller cannot probe
			// which addresses are registered.
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
			errResp := decodeAPIError(t, resp.Body)
			assert.Equal(t, "EMAIL_OR_PASSWORD_INCORRECT", errResp.Errors[0].Code)
			assert.Equal(t, "The email or the password is incorrect.", errResp.Errors[0].Message)
		},
	)

	t.Run(
		"MissingPassword", func(t *testing.T) {
			loginReq := ct.LoginJSONRequestBody{
				Email:    testOwnerUser(t).Email,
				Password: "",
			}
			resp, err := testTenant(t).NoAuthClient.LoginWithResponse(
				context.Background(), loginReq,
			)
			require.NoError(t, err, "login request should not error")
			assert.NotNil(t, resp.JSON400)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
			assert.Equal(t, "VALIDATION_ERROR", resp.JSON400.Errors[0].Code)
			assert.Equal(t, "Password is a required field", resp.JSON400.Errors[0].Message)
		},
	)
}
