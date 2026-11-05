package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeAPIError(t *testing.T, body []byte) ct.ApiErrorResponse {
	t.Helper()
	var errResp ct.ApiErrorResponse
	require.NoError(t, json.Unmarshal(body, &errResp), "error body should decode")
	require.NotEmpty(t, errResp.Errors, "error body should carry at least one error")
	return errResp
}

func TestLogin(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, testOwnerUser(t).Email)
	assert.NotEmpty(t, testOwnerUser(t).Password)

	t.Run(
		"Valid login credentials return access and refresh tokens", func(t *testing.T) {
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
		"Login refuses an incorrect password without authenticating the caller", func(t *testing.T) {
			loginReq := ct.LoginJSONRequestBody{
				Email:    testOwnerUser(t).Email,
				Password: "WrongPassword!",
			}
			resp, err := testTenant(t).NoAuthClient.LoginWithResponse(
				context.Background(), loginReq,
			)
			require.NoError(t, err, "login request should not error")
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
			errResp := decodeAPIError(t, resp.Body)
			assert.Equal(t, "EMAIL_OR_PASSWORD_INCORRECT", errResp.Errors[0].Code)
			assert.Equal(t, "The email or the password is incorrect.", errResp.Errors[0].Message)
		},
	)

	t.Run(
		"Login refuses an email address with no registered user", func(t *testing.T) {
			loginReq := ct.LoginJSONRequestBody{
				Email:    "doesnotexist@example.com",
				Password: "AnyPassword123!",
			}
			resp, err := testTenant(t).NoAuthClient.LoginWithResponse(
				context.Background(), loginReq,
			)
			require.NoError(t, err, "login request should not error")
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode())
			errResp := decodeAPIError(t, resp.Body)
			assert.Equal(t, "EMAIL_OR_PASSWORD_INCORRECT", errResp.Errors[0].Code)
			assert.Equal(t, "The email or the password is incorrect.", errResp.Errors[0].Message)
		},
	)

	t.Run(
		"Login refuses a request without a password", func(t *testing.T) {
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

func TestLoginTakesAsLongForAnUnknownEmail(t *testing.T) {
	t.Run("A wrong password and an unknown email take about the same time", func(t *testing.T) {
		known := createPlatformAdmin(t)
		client := testTenant(t).NoAuthClient
		unknown := "absent-" + uuid.NewString() + "@example.test"
		login := func(email string) time.Duration {
			started := time.Now()
			resp, err := client.LoginWithResponse(context.Background(),
				ct.LoginJSONRequestBody{Email: email, Password: "WrongPassword!1"})
			elapsed := time.Since(started)
			require.NoError(t, err)
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode())
			return elapsed
		}
		median := func(email string) time.Duration {
			samples := make([]time.Duration, 15)
			for i := range samples {
				samples[i] = login(email)
			}
			slices.Sort(samples)
			return samples[len(samples)/2]
		}
		login(known.Email)
		login(unknown)

		knownMedian, unknownMedian := median(known.Email), median(unknown)

		assert.Greater(t, unknownMedian, knownMedian/2,
			"known %s, unknown %s: an unknown email must not skip the password hash", knownMedian, unknownMedian)
	})
}
