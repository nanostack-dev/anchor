package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInvitation_ReturnsNoToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	resp := w.invitations.GetRaw(w.organizationID, created.Id)

	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	assert.NotContains(t, string(resp.Body), created.Token)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(resp.Body, &fields))
	assert.NotContains(t, fields, "token")
	assert.NotContains(t, fields, "token_hash")
	assert.Equal(t, created.Id, resp.JSON200.Id)
}

func TestGetInvitation_ReadsExpiredOnceTheExpiryHasPassed(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	require.Equal(t, ct.Pending, w.invitations.Get(w.organizationID, created.Id).Status)

	w.expire(created.Id)

	assert.Equal(t, ct.Expired, w.invitations.Get(w.organizationID, created.Id).Status)
}

func TestGetInvitation_NeverStoresExpired(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.expire(created.Id)
	require.Equal(t, ct.Expired, w.invitations.Get(w.organizationID, created.Id).Status)

	var statusColumns int
	require.NoError(t, testDB.QueryRow(
		`SELECT COUNT(*) FROM information_schema.columns
		 WHERE table_name = 'organization_invitations' AND column_name = 'status'`,
	).Scan(&statusColumns))
	assert.Zero(t, statusColumns)
	var acceptedAt *string
	require.NoError(t, testDB.QueryRow(
		"SELECT accepted_at::text FROM organization_invitations WHERE id = $1", created.Id,
	).Scan(&acceptedAt))
	assert.Nil(t, acceptedAt)
}

func TestGetInvitation_RefusesUnknownInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.invitations.GetRaw(w.organizationID, ids.MustNew("oinv"))

	require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_NOT_FOUND", errorCode(t, resp.JSON404.Errors))
}

func TestGetInvitation_RefusesInvitationOfAnotherOrganization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	resp := w.invitations.GetRaw(w.newOrganization(), created.Id)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
}

func TestSearchInvitations_ListsOnlyTheInvitationsOfTheOrganization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	mine := w.invite(uniqueEmail())
	otherOrganization := w.newOrganization()
	w.invitations.Create(otherOrganization, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(uniqueEmail()),
		RoleId: w.roleID,
	})

	items := w.invitations.Search(w.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{})

	require.Len(t, items, 1)
	assert.Equal(t, mine.Id, items[0].Id)
}

func TestSearchInvitations_ReturnsNoToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	resp := w.invitations.SearchRaw(w.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{})

	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	assert.NotContains(t, string(resp.Body), created.Token)
	assert.NotContains(t, string(resp.Body), `"token"`)
}

func TestSearchInvitations_ReadsExpiredOnceTheExpiryHasPassed(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.expire(created.Id)

	items := w.invitations.Search(w.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{})

	require.Len(t, items, 1)
	assert.Equal(t, ct.Expired, items[0].Status)
}

func TestSearchInvitations_FilterByPendingKeepsOnlyPendingAndDropsExpired(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	pending := w.invite(uniqueEmail())
	expired := w.invite(uniqueEmail())
	w.expire(expired.Id)

	items := w.invitations.SearchByStatus(w.organizationID, ct.Pending)

	require.Len(t, items, 1)
	assert.Equal(t, pending.Id, items[0].Id)
}

func TestSearchInvitations_FilterByExpiredKeepsAPendingInvitationWhoseExpiryPassed(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.invite(uniqueEmail())
	expired := w.invite(uniqueEmail())
	w.expire(expired.Id)

	items := w.invitations.SearchByStatus(w.organizationID, ct.Expired)

	require.Len(t, items, 1)
	assert.Equal(t, expired.Id, items[0].Id)
	assert.Equal(t, ct.Expired, items[0].Status)
}

func TestSearchInvitations_FilterByAcceptedKeepsOnlyAccepted(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.invite(uniqueEmail())
	accepting := w.invite(uniqueEmail())
	w.invitations.Accept(accepting.Token, w.newProductUser(string(accepting.Email)))

	items := w.invitations.SearchByStatus(w.organizationID, ct.Accepted)

	require.Len(t, items, 1)
	assert.Equal(t, accepting.Id, items[0].Id)
	assert.NotNil(t, items[0].AcceptedAt)
}

func TestSearchInvitations_FilterByTwoStatusesKeepsBoth(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	pending := w.invite(uniqueEmail())
	expired := w.invite(uniqueEmail())
	w.expire(expired.Id)
	accepting := w.invite(uniqueEmail())
	w.invitations.Accept(accepting.Token, w.newProductUser(uniqueEmail()))

	items := w.invitations.SearchByStatus(w.organizationID, ct.Pending, ct.Expired)

	assert.ElementsMatch(t, []string{pending.Id, expired.Id}, invitationIDs(items))
}

func TestSearchInvitations_RefusesUnknownOrganization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.invitations.SearchRaw(ids.MustNew("org"), ct.SearchOrganizationInvitationsJSONRequestBody{})

	assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
}

func TestSearchInvitations_RefusesUnknownStatus(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp, err := w.product.AllScopeAPIKeyClient().SearchOrganizationInvitationsWithBodyWithResponse(
		context.Background(), w.product.ProductID, w.organizationID, "application/json",
		strings.NewReader(`{"filter":{"statuses":["revoked"]}}`),
	)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
}
