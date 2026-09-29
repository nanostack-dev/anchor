package ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateInvitation_ChangesRoleAndExpiry(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	otherRole := w.newRole()
	expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Second)

	updated := w.invitations.Update(w.organizationID, created.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
		RoleId:    otherRole,
		ExpiresAt: expiresAt,
	})

	assert.Equal(t, otherRole, updated.RoleId)
	assert.WithinDuration(t, expiresAt, updated.ExpiresAt, time.Second)
	assert.Equal(t, created.Email, updated.Email)
	assert.Equal(t, otherRole, w.invitations.Get(w.organizationID, created.Id).RoleId)
}

func TestUpdateInvitation_IgnoresAnEmailInTheBody(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	expiresAt := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)

	resp := w.invitations.UpdateRawBody(
		w.organizationID, created.Id,
		`{"email":"`+uniqueEmail()+`","role_id":"`+w.roleID+`","expires_at":"`+expiresAt+`"}`,
	)

	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, created.Email, w.invitations.Get(w.organizationID, created.Id).Email)
}

func TestUpdateInvitation_KeepsTheTokenValid(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	w.invitations.Update(w.organizationID, created.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
		RoleId:    w.roleID,
		ExpiresAt: time.Now().Add(time.Hour),
	})

	assert.Equal(t, created.Id, w.invitations.Lookup(created.Token).Id)
}

func TestUpdateInvitation_RefusesExpiryInThePast(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	resp := w.invitations.UpdateRaw(w.organizationID, created.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
		RoleId:    w.roleID,
		ExpiresAt: time.Now().Add(-time.Minute),
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_EXPIRY_NOT_IN_FUTURE", errorCode(t, resp.JSON400.Errors))
}

func TestUpdateInvitation_RefusesUnknownRole(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	resp := w.invitations.UpdateRaw(w.organizationID, created.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
		RoleId:    ids.MustNew("product_role"),
		ExpiresAt: time.Now().Add(time.Hour),
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ROLE_NOT_FOUND_IN_REQUEST", errorCode(t, resp.JSON400.Errors))
}

func TestUpdateInvitation_RefusesUnknownInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.invitations.UpdateRaw(
		w.organizationID,
		ids.MustNew("oinv"),
		ct.UpdateOrganizationInvitationJSONRequestBody{
			RoleId:    w.roleID,
			ExpiresAt: time.Now().Add(time.Hour),
		},
	)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
}

func TestUpdateInvitation_RefusesAnExpiredInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.expire(created.Id)

	resp := w.invitations.UpdateRaw(w.organizationID, created.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
		RoleId:    w.roleID,
		ExpiresAt: time.Now().Add(time.Hour),
	})

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_EXPIRED", errorCode(t, resp.JSON409.Errors))
	assert.Equal(t, ct.Expired, w.invitations.Get(w.organizationID, created.Id).Status)
}

func TestUpdateInvitation_RefusesAnAcceptedInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.invitations.Accept(created.Token, w.newProductUser(uniqueEmail()))

	resp := w.invitations.UpdateRaw(w.organizationID, created.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
		RoleId:    w.newRole(),
		ExpiresAt: time.Now().Add(time.Hour),
	})

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_ALREADY_ACCEPTED", errorCode(t, resp.JSON409.Errors))
	assert.Equal(t, w.roleID, w.invitations.Get(w.organizationID, created.Id).RoleId)
}

func TestDeleteInvitation_RemovesTheInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	w.invitations.Delete(w.organizationID, created.Id)

	resp := w.invitations.GetRaw(w.organizationID, created.Id)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	assert.Empty(t, w.invitations.Search(w.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{}))
}

func TestDeleteInvitation_KillsTheToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	w.invitations.Delete(w.organizationID, created.Id)

	assert.Equal(t, http.StatusBadRequest, w.invitations.LookupRaw(created.Token).StatusCode())
}

func TestDeleteInvitation_FreesTheEmailForANewInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	created := w.invite(email)

	w.invitations.Delete(w.organizationID, created.Id)

	assert.Equal(t, ct.Pending, w.invite(email).Status)
}

func TestDeleteInvitation_RefusesUnknownInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.invitations.DeleteRaw(w.organizationID, ids.MustNew("oinv"))

	assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
}

func TestDeleteInvitation_RefusesInvitationOfAnotherOrganization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	resp := w.invitations.DeleteRaw(w.newOrganization(), created.Id)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, created.Id, w.invitations.Get(w.organizationID, created.Id).Id)
}

func TestResendInvitation_ReturnsANewTokenAndResetsTheExpiry(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	expiresAt := time.Now().Add(time.Hour)
	created := w.invitations.Create(w.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:     openapi_types.Email(uniqueEmail()),
		RoleId:    w.roleID,
		ExpiresAt: &expiresAt,
	})

	resent := w.invitations.Resend(w.organizationID, created.Id)

	assert.NotEqual(t, created.Token, resent.Token)
	assert.Contains(t, resent.Token, "anchor_inv_")
	assert.Equal(t, created.Id, resent.Id)
	assert.WithinDuration(t, time.Now().Add(defaultExpiry), resent.ExpiresAt, time.Minute)
}

func TestResendInvitation_KillsTheOldToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	userID := w.newProductUser(uniqueEmail())
	w.invitations.Resend(w.organizationID, created.Id)

	resp := w.invitations.AcceptRaw(created.Token, userID)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_TOKEN_NOT_FOUND", errorCode(t, resp.JSON400.Errors))
	w.assertNotMember(userID)
}

func TestResendInvitation_NewTokenAccepts(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	userID := w.newProductUser(uniqueEmail())
	resent := w.invitations.Resend(w.organizationID, created.Id)

	accepted := w.invitations.Accept(resent.Token, userID)

	assert.Equal(t, ct.Accepted, accepted.Status)
	assert.Equal(t, http.StatusOK, w.memberRaw(w.organizationID, userID).StatusCode())
}

func TestResendInvitation_KeepsTheInvitationPending(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	w.invitations.Resend(w.organizationID, created.Id)

	got := w.invitations.Get(w.organizationID, created.Id)
	assert.Equal(t, ct.Pending, got.Status)
	assert.Equal(t, created.Email, got.Email)
}

func TestResendInvitation_RefusesAnExpiredInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.expire(created.Id)

	resp := w.invitations.ResendRaw(w.organizationID, created.Id)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_EXPIRED", errorCode(t, resp.JSON409.Errors))
	assert.Equal(t, http.StatusOK, w.invitations.LookupRaw(created.Token).StatusCode())
}

func TestResendInvitation_RefusesAnAcceptedInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.invitations.Accept(created.Token, w.newProductUser(uniqueEmail()))

	resp := w.invitations.ResendRaw(w.organizationID, created.Id)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_ALREADY_ACCEPTED", errorCode(t, resp.JSON409.Errors))
}

func TestResendInvitation_RefusesUnknownInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.invitations.ResendRaw(w.organizationID, ids.MustNew("oinv"))

	assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
}
