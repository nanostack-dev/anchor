package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/nanostack-dev/shared/toolkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPlatformInvitation(t *testing.T) {
	t.Run(
		"ValidInvitation", func(t *testing.T) {
			// Create invitation first
			inviteReq := ct.CreatePlatformInvitationJSONRequestBody{
				Email: "invitee_get@example.com",
			}
			createResp, err := testOwnerClient(t).CreatePlatformInvitationWithResponse(
				context.Background(),
				inviteReq,
			)
			require.NoError(t, err, "invitation creation should not error")
			assert.Equal(t, http.StatusCreated, createResp.StatusCode())
			assert.NotNil(t, createResp.JSON201)
			invitationID := createResp.JSON201.Id

			// Get invitation
			getResp, err := testOwnerClient(t).GetPlatformInvitationWithResponse(
				context.Background(), invitationID,
			)
			require.NoError(t, err, "get invitation should not error")
			assert.Equal(t, http.StatusOK, getResp.StatusCode())
			assert.NotNil(t, getResp.JSON200)
			assert.Equal(t, "invitee_get@example.com", getResp.JSON200.Email)
		},
	)

	t.Run(
		"NonexistentInvitation", func(t *testing.T) {
			getResp, err := testOwnerClient(t).GetPlatformInvitationWithResponse(
				context.Background(), toolkit.NewID("pinv"),
			)
			require.NoError(t, err, "get invitation should not error")
			assert.Equal(t, http.StatusNotFound, getResp.StatusCode())
		},
	)

	t.Run(
		"InvalidInvitationID", func(t *testing.T) {
			getResp, err := testOwnerClient(t).GetPlatformInvitationWithResponse(
				context.Background(), "invalid-id-format",
			)
			require.NoError(t, err, "get invitation should not error")
			assert.Equal(t, http.StatusBadRequest, getResp.StatusCode())
		},
	)
}
