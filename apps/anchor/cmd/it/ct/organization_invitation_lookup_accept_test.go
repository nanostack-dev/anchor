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

func TestLookupInvitation_FindsTheInvitationByItsToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	found := w.invitations.Lookup(created.Token)

	assert.Equal(t, created.Id, found.Id)
	assert.Equal(t, created.Email, found.Email)
	assert.Equal(t, w.organizationID, found.OrganizationId)
}

func TestLookupInvitation_ReturnsNoToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	resp := w.invitations.LookupRaw(created.Token)

	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	assert.NotContains(t, string(resp.Body), created.Token)
	assert.NotContains(t, string(resp.Body), `"token"`)
}

func TestLookupInvitation_ReadsExpiredOnceTheExpiryHasPassed(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.expire(created.Id)

	assert.Equal(t, ct.Expired, w.invitations.Lookup(created.Token).Status)
}

func TestLookupInvitation_RefusesUnknownToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.invitations.LookupRaw("anchor_inv_unknown")

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_TOKEN_NOT_FOUND", errorCode(t, resp.JSON400.Errors))
}

func TestAcceptInvitation_CreatesTheMembershipWithTheInvitedRole(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	userID := w.newProductUser(uniqueEmail())

	accepted := w.invitations.Accept(created.Token, userID)

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

	accepted := w.invitations.Accept(created.Token, userID)

	assert.Equal(t, ct.Accepted, accepted.Status)
	assert.Equal(t, http.StatusOK, w.memberRaw(w.organizationID, userID).StatusCode())
}

func TestAcceptInvitation_RefusesUnknownToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	userID := w.newProductUser(uniqueEmail())

	resp := w.invitations.AcceptRaw("anchor_inv_unknown", userID)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_TOKEN_NOT_FOUND", errorCode(t, resp.JSON400.Errors))
	w.assertNotMember(userID)
}

func TestAcceptInvitation_RefusesAnAcceptedInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.invitations.Accept(created.Token, w.newProductUser(uniqueEmail()))
	secondUser := w.newProductUser(uniqueEmail())

	resp := w.invitations.AcceptRaw(created.Token, secondUser)

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

	resp := w.invitations.AcceptRaw(created.Token, userID)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_EXPIRED", errorCode(t, resp.JSON409.Errors))
	w.assertNotMember(userID)
	assert.Nil(t, w.invitations.Get(w.organizationID, created.Id).AcceptedAt)
}

func TestAcceptInvitation_RefusesUnknownProductUser(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	resp := w.invitations.AcceptRaw(created.Token, ids.MustNew("pusr"))

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

	resp := w.invitations.AcceptRaw(created.Token, foreignUser)

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

	w.invitations.AcceptRaw(created.Token, ids.MustNew("pusr"))

	assert.Equal(t, usersBefore, w.productUserCount())
}

func TestAcceptInvitation_RefusesAProductUserWhoIsAlreadyAMember(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	userID := w.newProductUser(uniqueEmail())
	w.addMember(userID)

	resp := w.invitations.AcceptRaw(created.Token, userID)

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

	w.invitations.Accept(first.Token, userID)
	w.invitations.Accept(second.Token, userID)

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
				t.Context(), w.product.ProductID,
				ct.AcceptOrganizationInvitationJSONRequestBody{Token: created.Token, ProductUserId: userIDs[attempt]},
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
