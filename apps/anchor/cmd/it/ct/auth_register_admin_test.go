package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	itshared "anchor/cmd/it/shared"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createPlatformInvitation is a helper function to create a platform invitation using the owner context.
func createPlatformInvitation(t *testing.T, email string) string {
	inviteReq := ct.CreatePlatformInvitationJSONRequestBody{
		Email: email,
	}
	resp, err := testOwnerClient(t).CreatePlatformInvitationWithResponse(
		context.Background(), inviteReq,
	)
	require.NoError(t, err, "invitation creation should not error")
	assert.Equal(t, http.StatusCreated, resp.StatusCode())
	assert.NotNil(t, resp.JSON201)

	// Return the invitation code
	return resp.JSON201.Code
}

func TestRegister(t *testing.T) {
	assert.NotEmpty(t, testOwnerUser(t).AccessToken)

	t.Run(
		"WeakPassword", func(t *testing.T) {
			registerReq := ct.RegisterJSONRequestBody{
				Email:    "register_weakpw@example.com",
				Password: "123",
			}
			resp, err := testTenant(t).NoAuthClient.RegisterWithResponse(
				context.Background(), registerReq,
			)
			require.NoError(t, err, "registration should not error")
			assert.NotNil(t, resp.JSON400)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
			assert.Equal(t, "VALIDATION_ERROR", resp.JSON400.Errors[0].Code)
			assert.Equal(
				t, "Password must be at least 8 characters in length",
				resp.JSON400.Errors[0].Message,
			)
		},
	)

	t.Run(
		"MissingFields", func(t *testing.T) {
			// Missing password
			registerReq := ct.RegisterJSONRequestBody{
				Email: "missingpw@example.com",
			}
			resp, err := testTenant(t).NoAuthClient.RegisterWithResponse(
				context.Background(), registerReq,
			)
			require.NoError(t, err, "registration should not error")
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
			assert.NotNil(t, resp.JSON400)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
			assert.Equal(t, "VALIDATION_ERROR", resp.JSON400.Errors[0].Code)
			assert.Equal(t, "Password is a required field", resp.JSON400.Errors[0].Message)
		},
	)

	t.Run(
		"WithInvitationCode", func(t *testing.T) {
			email := itshared.Faker.Internet().Email()
			// Setup: Create invitation using platform owner
			invitationCode := createPlatformInvitation(t, email)

			t.Run(
				"ValidInvitationCode", func(t *testing.T) {
					registerReq := ct.RegisterJSONRequestBody{
						Email:          email,
						Password:       "ValidPassword123!",
						InvitationCode: &invitationCode,
					}
					resp, err := testTenant(t).NoAuthClient.RegisterWithResponse(
						context.Background(), registerReq,
					)
					require.NoError(
						t, err, "registration with valid invitation code should not error",
					)
					assert.Equal(t, http.StatusOK, resp.StatusCode())
					assert.NotNil(t, resp.JSON200)
					assert.NotEmpty(t, resp.JSON200.AccessToken)
					assert.NotEmpty(t, resp.JSON200.RefreshToken)
				},
			)

			t.Run(
				"MissingInvitationCode", func(t *testing.T) {
					registerReq := ct.RegisterJSONRequestBody{
						Email:    itshared.Faker.Internet().Email(),
						Password: "ValidPassword123!",
					}
					resp, err := testTenant(t).NoAuthClient.RegisterWithResponse(
						context.Background(), registerReq,
					)
					require.NoError(t, err, "registration without invitation code should not error")
					assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
					assert.NotNil(t, resp.JSON400)
					assert.Equal(t, "INVITATION_CODE_NOT_PROVIDED", resp.JSON400.Errors[0].Code)
					assert.Equal(
						t, "The invitation code is required.",
						resp.JSON400.Errors[0].Message,
					)
				},
			)

			t.Run(
				"InvalidInvitationCode", func(t *testing.T) {
					email = itshared.Faker.Internet().Email()
					invitationCode = createPlatformInvitation(t, email)
					invalidCode := "invalid-code-123"
					registerReq := ct.RegisterJSONRequestBody{
						Email:          email,
						Password:       "ValidPassword123!",
						InvitationCode: &invalidCode,
					}
					resp, err := testTenant(t).NoAuthClient.RegisterWithResponse(
						context.Background(), registerReq,
					)
					require.NoError(
						t, err, "registration with invalid invitation code should not error",
					)
					assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
					assert.NotNil(t, resp.JSON400)
					assert.Equal(t, "INVITATION_CODE_IS_INVALID", resp.JSON400.Errors[0].Code)
					assert.Equal(
						t, "The invitation code is invalid.",
						resp.JSON400.Errors[0].Message,
					)
				},
			)
		},
	)
}
