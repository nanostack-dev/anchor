package ct_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestCreateInvitation_ReturnsTokenOnce(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()

	created := w.invite(email)

	assert.Contains(t, created.Token, "anchor_inv_")
	assert.Equal(t, email, string(created.Email))
	assert.Equal(t, w.organizationID, created.OrganizationId)
	assert.Equal(t, w.roleID, created.RoleId)
	assert.Equal(t, ct.Pending, created.Status)
	assert.Nil(t, created.AcceptedAt)
	assert.NotEmpty(t, created.Id)
}

func TestCreateInvitation_DefaultsExpiryToSevenDays(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	created := w.invite(uniqueEmail())

	assert.WithinDuration(t, time.Now().Add(defaultExpiry), created.ExpiresAt, time.Minute)
}

func TestCreateInvitation_AcceptsExplicitExpiry(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	expiresAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

	created := w.invitations.Create(w.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:     openapi_types.Email(uniqueEmail()),
		RoleId:    w.roleID,
		ExpiresAt: &expiresAt,
	})

	assert.WithinDuration(t, expiresAt, created.ExpiresAt, time.Second)
}

func TestCreateInvitation_RefusesExpiryInThePast(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	expiresAt := time.Now().Add(-time.Minute)

	resp := w.invitations.CreateRaw(w.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:     openapi_types.Email(uniqueEmail()),
		RoleId:    w.roleID,
		ExpiresAt: &expiresAt,
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_EXPIRY_NOT_IN_FUTURE", errorCode(t, resp.JSON400.Errors))
}

func TestCreateInvitation_RefusesSecondPendingForSameEmail(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	w.invite(email)

	resp := w.inviteRaw(email)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_ALREADY_PENDING", errorCode(t, resp.JSON409.Errors))
	assert.Len(t, w.invitations.Search(w.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{}), 1)
}

func TestCreateInvitation_ComparesEmailWithoutRegardToCase(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	local := "Casey.Mixed-" + itshared.Faker.UUID().V4()
	w.invite(local + "@Example.com")

	resp := w.inviteRaw(strings.ToLower(local) + "@example.COM")

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_ALREADY_PENDING", errorCode(t, resp.JSON409.Errors))
}

func TestCreateInvitation_AllowsNewInvitationAfterFirstExpired(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	first := w.invite(email)
	w.expire(first.Id)

	second := w.invite(email)

	assert.NotEqual(t, first.Id, second.Id)
	assert.Equal(t, ct.Pending, second.Status)
	assert.Equal(t, ct.Expired, w.invitations.Get(w.organizationID, first.Id).Status)
}

func TestCreateInvitation_AllowsSameEmailInAnotherOrganization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	w.invite(email)

	other := w.invitations.Create(w.newOrganization(), ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(email),
		RoleId: w.roleID,
	})

	assert.Equal(t, ct.Pending, other.Status)
}

func TestCreateInvitation_RefusesEmailOfExistingMember(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	w.addMember(w.newProductUser(email))

	resp := w.inviteRaw(email)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_EMAIL_IS_MEMBER", errorCode(t, resp.JSON409.Errors))
}

func TestCreateInvitation_ComparesMemberEmailWithoutRegardToCase(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	local := "Member.Mixed-" + itshared.Faker.UUID().V4()
	w.addMember(w.newProductUser(local + "@Example.com"))

	resp := w.inviteRaw(strings.ToLower(local) + "@example.com")

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_INVITATION_EMAIL_IS_MEMBER", errorCode(t, resp.JSON409.Errors))
}

func TestCreateInvitation_AllowsEmailOfMemberOfAnotherOrganization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	w.addMember(w.newProductUser(email))

	created := w.invitations.Create(w.newOrganization(), ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(email),
		RoleId: w.roleID,
	})

	assert.Equal(t, ct.Pending, created.Status)
}

func TestCreateInvitation_RefusesUnknownRole(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.invitations.CreateRaw(w.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(uniqueEmail()),
		RoleId: ids.MustNew("product_role"),
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ROLE_NOT_FOUND_IN_REQUEST", errorCode(t, resp.JSON400.Errors))
}

func TestCreateInvitation_RefusesRoleOfAnotherProduct(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	foreign := newWorld(t)

	resp := w.invitations.CreateRaw(w.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(uniqueEmail()),
		RoleId: foreign.roleID,
	})

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ROLE_NOT_FOUND_IN_REQUEST", errorCode(t, resp.JSON400.Errors))
}

func TestCreateInvitation_RefusesUnknownOrganization(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.invitations.CreateRaw(ids.MustNew("org"), ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(uniqueEmail()),
		RoleId: w.roleID,
	})

	require.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "ORGANIZATION_NOT_FOUND", errorCode(t, resp.JSON404.Errors))
}

func TestCreateInvitation_RefusesMalformedEmail(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.invitations.CreateRawBody(
		w.organizationID, `{"email":"not-an-email","role_id":"`+w.roleID+`"}`,
	)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "BAD_REQUEST", errorCode(t, resp.JSON400.Errors))
}

func TestCreateInvitation_StoresOnlyTheHashOfTheToken(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	created := w.invite(uniqueEmail())

	var storedHash string
	require.NoError(t, testDB.QueryRow(
		"SELECT token_hash FROM organization_invitations WHERE id = $1", created.Id,
	).Scan(&storedHash))
	assert.NotEmpty(t, storedHash)
	assert.NotEqual(t, created.Token, storedHash)
	assert.NotContains(t, storedHash, created.Token)
}

func TestCreateInvitation_HoldsOnePendingPerEmailUnderConcurrentCreates(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	email := uniqueEmail()
	const attempts = 8

	statuses := make([]int, attempts)
	var wg sync.WaitGroup
	for attempt := range attempts {
		wg.Go(func() {
			resp, err := w.product.AllScopeAPIKeyClient().CreateOrganizationInvitationWithResponse(
				t.Context(), w.product.ProductID, w.organizationID,
				ct.CreateOrganizationInvitationJSONRequestBody{
					Email:  openapi_types.Email(email),
					RoleId: w.roleID,
				},
			)
			if err == nil {
				statuses[attempt] = resp.StatusCode()
			}
		})
	}
	wg.Wait()

	created, refused := 0, 0
	for _, status := range statuses {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			refused++
		}
	}
	assert.Equal(t, 1, created, "statuses: %v", statuses)
	assert.Equal(t, attempts-1, refused, "statuses: %v", statuses)
	assert.Len(t, w.invitations.Search(w.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{}), 1)
}
