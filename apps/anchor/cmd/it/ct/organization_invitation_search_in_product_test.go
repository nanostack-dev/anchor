package ct_test

import (
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchInvitationsInProduct_FindsThePendingInvitationsOfAnEmailAcrossOrganizations(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	first := w.invite(email)
	otherOrganization := w.newOrganization()
	second := w.invitations.Create(otherOrganization, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(email),
		RoleId: w.roleID,
	})

	found := w.invitations.PendingFor(email)

	assert.ElementsMatch(t, []string{first.Id, second.Id}, invitationIDs(found))
}

func TestSearchInvitationsInProduct_ComparesEmailsWithoutLetterCase(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	created := w.invite(strings.ToUpper(email))

	found := w.invitations.PendingFor(email)

	assert.Equal(t, []string{created.Id}, invitationIDs(found))
}

func TestSearchInvitationsInProduct_MatchesAnyOfTheEmails(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	primary := w.invite(uniqueEmail())
	secondary := w.invite(uniqueEmail())
	w.invite(uniqueEmail())

	found := w.invitations.PendingFor(string(primary.Email), string(secondary.Email))

	assert.ElementsMatch(t, []string{primary.Id, secondary.Id}, invitationIDs(found))
}

func TestSearchInvitationsInProduct_LeavesOutAcceptedAndExpiredInvitationsWhenAskedForPending(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	accepted := w.invite(email)
	w.accept(accepted.Id, w.newProductUser(uniqueEmail()))
	expiredOrganization := w.newOrganization()
	expired := w.invitations.Create(expiredOrganization, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(email),
		RoleId: w.roleID,
	})
	w.expire(expired.Id)
	pendingOrganization := w.newOrganization()
	pending := w.invitations.Create(pendingOrganization, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(email),
		RoleId: w.roleID,
	})

	found := w.invitations.PendingFor(email)

	assert.Equal(t, []string{pending.Id}, invitationIDs(found))
}

func TestSearchInvitationsInProduct_ReadsEveryStatusWithoutAStatusFilter(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	accepted := w.invite(email)
	w.accept(accepted.Id, w.newProductUser(uniqueEmail()))

	found := w.invitations.SearchInProduct(ct.SearchProductOrganizationInvitationsJSONRequestBody{
		Filter: &ct.OrganizationInvitationFilter{Emails: &[]openapi_types.Email{openapi_types.Email(email)}},
	})

	require.Equal(t, []string{accepted.Id}, invitationIDs(found))
	assert.Equal(t, ct.Accepted, found[0].Status)
}

func TestSearchInvitationsInProduct_ReturnsTheOrganizationOfEachInvitation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())

	found := w.invitations.PendingFor(string(created.Email))

	require.Len(t, found, 1)
	assert.Equal(t, w.organizationID, found[0].OrganizationId)
}

func TestSearchInvitations_FiltersByEmail(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	wanted := w.invite(uniqueEmail())
	w.invite(uniqueEmail())

	found := w.invitations.SearchByEmail(w.organizationID, strings.ToUpper(string(wanted.Email)))

	assert.Equal(t, []string{wanted.Id}, invitationIDs(found))
}
