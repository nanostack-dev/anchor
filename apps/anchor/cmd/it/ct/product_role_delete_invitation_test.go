package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (w world) deleteRoleRaw(roleID string) *ct.DeleteProductRoleResponse {
	w.t.Helper()
	resp, err := w.product.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
		context.Background(), w.product.ProductID, roleID,
	)
	require.NoError(w.t, err)
	return resp
}

func (w world) assertRoleExists(roleID string) {
	w.t.Helper()
	resp, err := w.product.OwnerAuthenticatedClient().GetProductRoleWithResponse(
		context.Background(), w.product.ProductID, roleID,
	)
	require.NoError(w.t, err)
	assert.Equal(w.t, http.StatusOK, resp.StatusCode())
}

func (w world) assertInvitationGone(invitationID string) {
	w.t.Helper()
	resp := w.invitations.GetRaw(w.organizationID, invitationID)
	assert.Equal(w.t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
}

func TestDeleteProductRole_IsRefusedWhileAPendingInvitationNamesTheRole(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	resp := w.deleteRoleRaw(w.roleID)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ROLE_IN_USE", errorCode(t, resp.JSON409.Errors))
	w.assertRoleExists(w.roleID)
	assert.Equal(t, w.roleID, w.invitations.Get(w.organizationID, created.Id).RoleId)
}

func TestDeleteProductRole_IsAllowedOnceThePendingInvitationChangesRole(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	otherRole := w.newRole()
	w.invitations.Update(w.organizationID, created.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
		RoleId:    otherRole,
		ExpiresAt: time.Now().Add(time.Hour),
	})

	resp := w.deleteRoleRaw(w.roleID)

	require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, otherRole, w.invitations.Get(w.organizationID, created.Id).RoleId)
}

func TestDeleteProductRole_IsAllowedOnceThePendingInvitationIsDeleted(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.invitations.Delete(w.organizationID, created.Id)

	resp := w.deleteRoleRaw(w.roleID)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
}

func TestDeleteProductRole_DeletesAnExpiredInvitationAndEmitsItsDeletedEvent(t *testing.T) {
	t.Parallel()
	w, sink := newWorldCapturingEvents(t)
	created := w.invite(uniqueEmail())
	w.expire(created.Id)

	resp := w.deleteRoleRaw(w.roleID)

	require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	w.assertInvitationGone(created.Id)
	sink.WaitFor(eventInvitationDeleted, invitationFields(w, created.Id))
}

func TestDeleteProductRole_DeletesAnAcceptedInvitationAndEmitsItsDeletedEvent(t *testing.T) {
	t.Parallel()
	w, sink := newWorldCapturingEvents(t)
	created := w.invite(uniqueEmail())
	userID := w.newProductUser(uniqueEmail())
	w.invitations.Accept(created.Token, userID)
	removed, err := w.product.AllScopeAPIKeyClient().RemoveOrganizationMemberWithResponse(
		context.Background(), w.product.ProductID, w.organizationID, userID,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, removed.StatusCode(), string(removed.Body))

	resp := w.deleteRoleRaw(w.roleID)

	require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	w.assertInvitationGone(created.Id)
	sink.WaitFor(eventInvitationDeleted, invitationFields(w, created.Id))
}
