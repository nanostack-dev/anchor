package invitation_ct_test

import (
	"encoding/json"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

const (
	eventInvitationCreated  = "organization.invitation.created"
	eventInvitationUpdated  = "organization.invitation.updated"
	eventInvitationDeleted  = "organization.invitation.deleted"
	eventInvitationAccepted = "organization.invitation.accepted"
	eventMembershipCreated  = "organization.membership.created"
)

func newWorldCapturingEvents(t *testing.T) (world, *itdsl.EventSink) {
	t.Helper()
	w := newWorld(t)
	return w, w.product.CaptureEvents()
}

func invitationFields(w world, invitationID string) map[string]string {
	return map[string]string{
		"organization_id": w.organizationID,
		"invitation_id":   invitationID,
	}
}

func TestCreateInvitation_EmitsTheCreatedEvent(t *testing.T) {
	w, sink := newWorldCapturingEvents(t)

	created := w.invite(uniqueEmail())

	sink.WaitFor(eventInvitationCreated, invitationFields(w, created.Id))
}

func TestInvitationEvents_CarryIdentifiersOnly(t *testing.T) {
	w, sink := newWorldCapturingEvents(t)
	created := w.invite(uniqueEmail())

	event := sink.WaitFor(eventInvitationCreated, invitationFields(w, created.Id))

	var data map[string]string
	require.NoError(t, json.Unmarshal(event.Data, &data))
	assert.Equal(t, invitationFields(w, created.Id), data)
	assert.NotContains(t, string(event.Data), created.Token)
	assert.NotContains(t, string(event.Data), string(created.Email))
}

func TestUpdateInvitation_EmitsTheUpdatedEvent(t *testing.T) {
	w, sink := newWorldCapturingEvents(t)
	created := w.invite(uniqueEmail())

	w.invitations.Update(w.organizationID, created.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
		RoleId:    w.roleID,
		ExpiresAt: time.Now().Add(time.Hour),
	})

	sink.WaitFor(eventInvitationUpdated, invitationFields(w, created.Id))
}

func TestResendInvitation_EmitsTheUpdatedEvent(t *testing.T) {
	w, sink := newWorldCapturingEvents(t)
	created := w.invite(uniqueEmail())

	w.invitations.Resend(w.organizationID, created.Id)

	sink.WaitFor(eventInvitationUpdated, invitationFields(w, created.Id))
}

func TestDeleteInvitation_EmitsTheDeletedEvent(t *testing.T) {
	w, sink := newWorldCapturingEvents(t)
	created := w.invite(uniqueEmail())

	w.invitations.Delete(w.organizationID, created.Id)

	sink.WaitFor(eventInvitationDeleted, invitationFields(w, created.Id))
}

func TestAcceptInvitation_EmitsTheAcceptedEventAndTheMembershipCreatedEvent(t *testing.T) {
	w, sink := newWorldCapturingEvents(t)
	created := w.invite(uniqueEmail())
	userID := w.newProductUser(uniqueEmail())

	w.invitations.Accept(created.Token, userID)

	accepted := sink.WaitFor(eventInvitationAccepted, invitationFields(w, created.Id))
	assert.Equal(t, userID, accepted.Field("product_user_id"))
	sink.WaitFor(eventMembershipCreated, map[string]string{
		"organization_id": w.organizationID,
		"product_user_id": userID,
	})
}

func TestAcceptInvitation_EmitsNoEventWhenTheAcceptIsRefused(t *testing.T) {
	w, sink := newWorldCapturingEvents(t)
	created := w.invite(uniqueEmail())
	sink.WaitFor(eventInvitationCreated, invitationFields(w, created.Id))
	w.expire(created.Id)

	w.invitations.AcceptRaw(created.Token, w.newProductUser(uniqueEmail()))
	marker := w.invite(uniqueEmail())
	sink.WaitFor(eventInvitationCreated, invitationFields(w, marker.Id))

	assert.Zero(t, sink.Count(eventInvitationAccepted))
	assert.Zero(t, sink.Count(eventMembershipCreated))
}
