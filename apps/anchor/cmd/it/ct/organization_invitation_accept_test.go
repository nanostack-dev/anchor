package ct_test

import (
	"net/http"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcceptInvitation_CreatesTheMembershipWithTheInvitedRole(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	userID := w.newProductUser(uniqueEmail())

	accepted := w.accept(created.Id, userID)

	assert.Equal(t, ct.Accepted, accepted.Status)
	assert.NotNil(t, accepted.AcceptedAt)
	member := w.memberRaw(w.organizationID, userID)
	require.Equal(t, http.StatusOK, member.StatusCode(), string(member.Body))
	assert.Equal(t, w.roleID, member.JSON200.Role.Id)
	assert.Equal(t, ct.Accepted, w.invitations.Get(w.organizationID, created.Id).Status)
}

func TestAcceptInvitation_AcceptsWhenTheProductUserEmailDiffers(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite("invited@example.com")
	userID := w.newProductUser("someone.else@example.com")

	accepted := w.accept(created.Id, userID)

	assert.Equal(t, ct.Accepted, accepted.Status)
	assert.Equal(t, http.StatusOK, w.memberRaw(w.organizationID, userID).StatusCode())
}

func TestAcceptInvitation_RefusesUnknownInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	userID := w.newProductUser(uniqueEmail())

	resp := w.acceptRaw(ids.MustNew("oinv"), userID)

	require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_NOT_FOUND", errorCode(t, resp.JSON404.Errors))
	w.assertNotMember(userID)
}

func TestAcceptInvitation_RefusesInvitationOfAnotherOrganization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	otherOrganization := w.newOrganization()
	userID := w.newProductUser(uniqueEmail())

	resp := w.invitations.AcceptRaw(otherOrganization, created.Id, userID)

	require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, http.StatusNotFound, w.memberRaw(otherOrganization, userID).StatusCode())
	assert.Equal(t, ct.Pending, w.invitations.Get(w.organizationID, created.Id).Status)
}

func TestAcceptInvitation_RefusesAnAcceptedInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.accept(created.Id, w.newProductUser(uniqueEmail()))
	secondUser := w.newProductUser(uniqueEmail())

	resp := w.acceptRaw(created.Id, secondUser)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_ALREADY_ACCEPTED", errorCode(t, resp.JSON409.Errors))
	w.assertNotMember(secondUser)
}

func TestAcceptInvitation_RefusesAnExpiredInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.expire(created.Id)
	userID := w.newProductUser(uniqueEmail())

	resp := w.acceptRaw(created.Id, userID)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_EXPIRED", errorCode(t, resp.JSON409.Errors))
	w.assertNotMember(userID)
	assert.Nil(t, w.invitations.Get(w.organizationID, created.Id).AcceptedAt)
}

func TestAcceptInvitation_RefusesUnknownProductUser(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	resp := w.acceptRaw(created.Id, ids.MustNew("pusr"))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "PRODUCT_USER_NOT_FOUND_IN_REQUEST", errorCode(t, resp.JSON400.Errors))
	assert.Equal(t, ct.Pending, w.invitations.Get(w.organizationID, created.Id).Status)
}

func TestAcceptInvitation_RefusesProductUserOfAnotherProduct(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	foreign := newWorld(t)
	created := w.invite(uniqueEmail())
	foreignUser := foreign.newProductUser(uniqueEmail())

	resp := w.acceptRaw(created.Id, foreignUser)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "PRODUCT_USER_NOT_FOUND_IN_REQUEST", errorCode(t, resp.JSON400.Errors))
	assert.Equal(t, ct.Pending, w.invitations.Get(w.organizationID, created.Id).Status)
}

func TestAcceptInvitation_NeverCreatesAProductUser(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	created := w.invite(email)
	usersBefore := w.productUserCount()

	w.acceptRaw(created.Id, ids.MustNew("pusr"))

	assert.Equal(t, usersBefore, w.productUserCount())
}

func TestAcceptInvitation_RefusesAProductUserWhoIsAlreadyAMember(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	userID := w.newProductUser(uniqueEmail())
	w.addMember(userID)

	resp := w.acceptRaw(created.Id, userID)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_MEMBERSHIP_ALREADY_EXISTS", errorCode(t, resp.JSON409.Errors))
	assert.Equal(t, ct.Pending, w.invitations.Get(w.organizationID, created.Id).Status)
}

func TestAcceptInvitation_AllowsTheSameProductUserToJoinAnotherOrganization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	userID := w.newProductUser(uniqueEmail())
	first := w.invite(uniqueEmail())
	otherOrganization := w.newOrganization()
	second := w.invitations.Create(otherOrganization, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(uniqueEmail()),
		RoleId: w.roleID,
	})

	w.accept(first.Id, userID)
	w.invitations.Accept(otherOrganization, second.Id, userID)

	assert.Equal(t, http.StatusOK, w.memberRaw(w.organizationID, userID).StatusCode())
	assert.Equal(t, http.StatusOK, w.memberRaw(otherOrganization, userID).StatusCode())
}

func TestAcceptInvitation_CreatesOneMembershipUnderConcurrentAccepts(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	const attempts = 6
	userIDs := make([]string, attempts)
	for i := range userIDs {
		userIDs[i] = w.newProductUser(uniqueEmail())
	}

	statuses := make([]int, attempts)
	var wg sync.WaitGroup
	for attempt := range attempts {
		wg.Go(func() {
			resp, err := w.product.AllScopeAPIKeyClient().AcceptOrganizationInvitationWithResponse(
				t.Context(), w.product.ProductID, w.organizationID, created.Id,
				ct.AcceptOrganizationInvitationJSONRequestBody{ProductUserId: userIDs[attempt]},
			)
			if err == nil {
				statuses[attempt] = resp.StatusCode()
			}
		})
	}
	wg.Wait()

	accepted, refused := 0, 0
	for _, status := range statuses {
		switch status {
		case http.StatusOK:
			accepted++
		case http.StatusConflict:
			refused++
		}
	}
	assert.Equal(t, 1, accepted, "statuses: %v", statuses)
	assert.Equal(t, attempts-1, refused, "statuses: %v", statuses)
	members := 0
	for _, userID := range userIDs {
		if w.memberRaw(w.organizationID, userID).StatusCode() == http.StatusOK {
			members++
		}
	}
	assert.Equal(t, 1, members)
}
