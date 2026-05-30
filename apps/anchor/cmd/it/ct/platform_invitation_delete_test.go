package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeletePlatformInvitation(t *testing.T) {
	assert.NotEmpty(t, testOwnerUser(t).AccessToken)

	t.Run(
		"ValidDeletion", func(t *testing.T) {
			// Create invitation first
			inviteReq := ct.CreatePlatformInvitationJSONRequestBody{
				Email: "invitee_delete@example.com",
			}
			createResp, err := testOwnerClient(t).CreatePlatformInvitationWithResponse(
				context.Background(),
				inviteReq,
			)
			require.NoError(t, err, "invitation creation should not error")
			assert.Equal(t, http.StatusCreated, createResp.StatusCode())

			var invitationResp ct.PlatformInvitationResponse
			err = json.Unmarshal(createResp.Body, &invitationResp)
			require.NoError(t, err, "should unmarshal invitation response")
			invitationID := invitationResp.Id

			// Delete invitation
			delResp, err := testOwnerClient(t).DeletePlatformInvitationWithResponse(
				context.Background(), invitationID,
			)
			require.NoError(t, err, "invitation deletion should not error")
			assert.Equal(t, http.StatusNoContent, delResp.StatusCode())
		},
	)

	t.Run(
		"NonexistentInvitation", func(t *testing.T) {
			delResp, err := testOwnerClient(t).DeletePlatformInvitationWithResponse(
				context.Background(), ids.MustNew("pinv"),
			)
			require.NoError(t, err, "invitation deletion should not error")
			assert.Equal(t, http.StatusNoContent, delResp.StatusCode())
		},
	)

	t.Run(
		"InvalidInvitationID", func(t *testing.T) {
			delResp, err := testOwnerClient(t).DeletePlatformInvitationWithResponse(
				context.Background(), ct.Ksuid("invalid-id-format"),
			)
			require.NoError(t, err, "invitation deletion should not error")
			assert.Equal(t, http.StatusBadRequest, delResp.StatusCode())
		},
	)
}
