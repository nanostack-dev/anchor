package invitation_ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

func TestInvitations_AnotherProductCannotReachAnInvitationOfThisProduct(t *testing.T) {
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "t"}).
		Product(itdsl.ProductOpts{Alias: "owner", TenantAlias: "t"}).
		Product(itdsl.ProductOpts{Alias: "intruder", TenantAlias: "t"}).
		Build()
	owner := newWorldOn(t, state.Product("owner"))
	intruderProduct := state.Product("intruder")
	intruder := intruderProduct.Invitations()
	created := owner.invite(uniqueEmail())
	intruderUser := newWorldOn(t, intruderProduct).newProductUser(uniqueEmail())

	t.Run("get", func(t *testing.T) {
		resp := intruder.GetRaw(owner.organizationID, created.Id)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
	t.Run("search", func(t *testing.T) {
		resp := intruder.SearchRaw(owner.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
	t.Run("create", func(t *testing.T) {
		resp := intruder.CreateRaw(owner.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
			Email:  "intruder@example.com",
			RoleId: owner.roleID,
		})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
	t.Run("update", func(t *testing.T) {
		resp := intruder.UpdateRaw(owner.organizationID, created.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
			RoleId:    owner.roleID,
			ExpiresAt: time.Now().Add(time.Hour),
		})
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
	t.Run("resend", func(t *testing.T) {
		resp := intruder.ResendRaw(owner.organizationID, created.Id)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
	t.Run("delete", func(t *testing.T) {
		resp := intruder.DeleteRaw(owner.organizationID, created.Id)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
	})
	t.Run("lookup", func(t *testing.T) {
		resp := intruder.LookupRaw(created.Token)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	})
	t.Run("accept", func(t *testing.T) {
		resp := intruder.AcceptRaw(created.Token, intruderUser)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	})

	still := owner.invitations.Get(owner.organizationID, created.Id)
	require.Equal(t, ct.Pending, still.Status)
	assert.Equal(t, created.RoleId, still.RoleId)
	assert.Equal(t, created.ExpiresAt.Unix(), still.ExpiresAt.Unix())
	assert.Equal(t, created.Id, owner.invitations.Lookup(created.Token).Id)
	assert.Len(t, owner.invitations.Search(owner.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{}), 1)
}

func TestInvitations_APlatformOwnerOfAnotherTenantCannotReachThem(t *testing.T) {
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	otherTenant := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "other", Isolated: true}).
		Build().Tenant("other")

	resp, err := otherTenant.OwnerClient.GetOrganizationInvitationWithResponse(
		t.Context(), w.product.ProductID, w.organizationID, created.Id,
	)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode(), string(resp.Body))
}
