package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			assert.NotNil(t, resp.JSON400)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
			assert.Equal(t, "EMAIL_OR_PASSWORD_INCORRECT", resp.JSON400.Errors[0].Code)
			assert.Equal(t, "Email or password is incorrect", resp.JSON400.Errors[0].Message)
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
			assert.NotNil(t, resp.JSON400)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
			assert.Equal(t, "EMAIL_OR_PASSWORD_INCORRECT", resp.JSON400.Errors[0].Code)
			assert.Equal(t, "Email or password is incorrect", resp.JSON400.Errors[0].Message)
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
