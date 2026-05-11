package ct_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchPlatformInvitations(t *testing.T) {
	assert.NotEmpty(t, testOwnerUser(t).AccessToken)

	t.Run(
		"ValidSearch", func(t *testing.T) {
			// Create test invitations
			for i := range 2 {
				email := fmt.Sprintf("invitee_search%c@example.com", 'A'+i)
				inviteReq := ct.CreatePlatformInvitationJSONRequestBody{
					Email: email,
				}
				_, err := testOwnerClient(t).CreatePlatformInvitationWithResponse(
					context.Background(), inviteReq,
				)
				require.NoError(t, err, "invitation creation should not error")
			}

			// Search invitations
			searchReq := ct.SearchPlatformInvitationsJSONRequestBody{
				Filter:         &ct.PlatformInvitationFilter{},
				FullTextSearch: func() *string { s := "invitee_search"; return &s }(),
			}
			searchResp, err := testOwnerClient(t).SearchPlatformInvitationsWithResponse(
				context.Background(),
				searchReq,
			)
			require.NoError(t, err, "search invitations should not error")
			assert.Equal(t, http.StatusOK, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.GreaterOrEqual(t, len(searchResp.JSON200.Items), 2)
		},
	)

	t.Run(
		"EmptySearch", func(t *testing.T) {
			searchReq := ct.SearchPlatformInvitationsJSONRequestBody{
				Filter: &ct.PlatformInvitationFilter{},
			}
			searchResp, err := testOwnerClient(t).SearchPlatformInvitationsWithResponse(
				context.Background(),
				searchReq,
			)
			require.NoError(t, err, "search invitations should not error")
			assert.Equal(t, http.StatusOK, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
		},
	)

	t.Run(
		"NoResults", func(t *testing.T) {
			searchReq := ct.SearchPlatformInvitationsJSONRequestBody{
				Filter:         &ct.PlatformInvitationFilter{},
				FullTextSearch: func() *string { s := "nonexistent-search-term-12345"; return &s }(),
			}
			searchResp, err := testOwnerClient(t).SearchPlatformInvitationsWithResponse(
				context.Background(),
				searchReq,
			)
			require.NoError(t, err, "search invitations should not error")
			assert.Equal(t, http.StatusOK, searchResp.StatusCode())
			assert.NotNil(t, searchResp.JSON200)
			assert.Empty(t, searchResp.JSON200.Items)
		},
	)
}
