package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePlatformInvitation(t *testing.T) {
	assert.NotEmpty(t, testOwnerUser(t).AccessToken)

	t.Run(
		"ValidInvitation", func(t *testing.T) {
			inviteReq := ct.CreatePlatformInvitationJSONRequestBody{
				Email: "invitee_create@example.com",
			}
			resp, err := testOwnerClient(t).CreatePlatformInvitationWithResponse(
				context.Background(), inviteReq,
			)
			require.NoError(t, err, "invitation creation should not error")
			assert.NotNil(t, resp)
			assert.Equal(t, http.StatusCreated, resp.StatusCode())
			assert.NotNil(t, resp.JSON201)
			assert.Equal(t, "invitee_create@example.com", resp.JSON201.Email)
		},
	)

	t.Run(
		"DuplicateInvitation", func(t *testing.T) {
			// First invitation (should succeed)
			inviteReq := ct.CreatePlatformInvitationJSONRequestBody{
				Email: "invitee_duplicate@example.com",
			}
			resp, err := testOwnerClient(t).CreatePlatformInvitationWithResponse(
				context.Background(), inviteReq,
			)
			require.NoError(t, err, "first invitation should not error")
			assert.Equal(t, http.StatusCreated, resp.StatusCode())

			// Second invitation for same email (should fail)
			resp, err = testOwnerClient(t).CreatePlatformInvitationWithResponse(
				context.Background(), inviteReq,
			)
			require.NoError(t, err, "second invitation should not error")
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode())
		},
	)
}
