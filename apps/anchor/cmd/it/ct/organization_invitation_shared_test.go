package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
)

const defaultExpiry = 7 * 24 * time.Hour

type world struct {
	t              *testing.T
	product        *itdsl.ProductContext
	invitations    itdsl.InvitationClient
	organizationID string
	roleID         string
}

func newWorld(t *testing.T) world {
	t.Helper()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "t"}).
		Product(itdsl.ProductOpts{Alias: "p", TenantAlias: "t"}).
		Build()
	return newWorldOn(t, state.Product("p"))
}

func newWorldOn(t *testing.T, product *itdsl.ProductContext) world {
	t.Helper()
	w := world{t: t, product: product, invitations: product.Invitations()}
	w.organizationID = w.newOrganization()
	w.roleID = w.newRole()
	return w
}

func (w world) newOrganization() string {
	w.t.Helper()
	return w.product.Organizations().
		Create(ct.CreateProductOrganizationJSONRequestBody{Name: itdsl.UniqueOrganizationName()}).Id
}

func (w world) newRole() string {
	w.t.Helper()
	resp, err := w.product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
		context.Background(), w.product.ProductID,
		ct.CreateProductRoleJSONRequestBody{Name: "role-" + itshared.Faker.UUID().V4()},
	)
	require.NoError(w.t, err)
	require.Equal(w.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return resp.JSON201.Id
}

func (w world) newProductUser(email string) string {
	w.t.Helper()
	alias := "user-" + itshared.Faker.UUID().V4()
	state := itdsl.Given(w.t).
		ExistingProduct(itdsl.ExistingProductOpts{Alias: "p", Context: w.product}).
		ProductUser(itdsl.ProductUserOpts{Alias: alias, ProductAlias: "p", Email: email}).
		Build()
	return state.ProductUser(alias).ID
}

func uniqueEmail() string {
	return "invitee-" + itshared.Faker.UUID().V4() + "@example.com"
}

func (w world) invite(email string) ct.OrganizationInvitationResponse {
	w.t.Helper()
	return w.invitations.Create(w.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(email),
		RoleId: w.roleID,
	})
}

func (w world) accept(invitationID, productUserID string) ct.OrganizationInvitationResponse {
	w.t.Helper()
	return w.invitations.Accept(w.organizationID, invitationID, productUserID)
}

func (w world) acceptRaw(invitationID, productUserID string) *ct.AcceptOrganizationInvitationResponse {
	w.t.Helper()
	return w.invitations.AcceptRaw(w.organizationID, invitationID, productUserID)
}

func (w world) inviteRaw(email string) *ct.CreateOrganizationInvitationResponse {
	w.t.Helper()
	return w.invitations.CreateRaw(w.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:  openapi_types.Email(email),
		RoleId: w.roleID,
	})
}

func (w world) addMember(productUserID string) {
	w.t.Helper()
	resp, err := w.product.AllScopeAPIKeyClient().AddOrganizationMemberWithResponse(
		context.Background(), w.product.ProductID, w.organizationID,
		ct.AddOrganizationMemberJSONRequestBody{ProductUserId: productUserID, RoleId: w.roleID},
	)
	require.NoError(w.t, err)
	require.Equal(w.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
}

func (w world) memberRaw(organizationID, productUserID string) *ct.GetOrganizationMemberResponse {
	w.t.Helper()
	resp, err := w.product.AllScopeAPIKeyClient().GetOrganizationMemberWithResponse(
		context.Background(), w.product.ProductID, organizationID, productUserID, &ct.GetOrganizationMemberParams{},
	)
	require.NoError(w.t, err)
	return resp
}

func (w world) assertNotMember(productUserID string) {
	w.t.Helper()
	assert.Equal(w.t, http.StatusNotFound, w.memberRaw(w.organizationID, productUserID).StatusCode())
}

// expire moves the expiry into the past, the only way to reach an expired
// invitation: the API refuses an expiry that is not in the future.
func (w world) expire(invitationID string) {
	w.t.Helper()
	result, err := testDB.ExecContext(
		context.Background(),
		"UPDATE organization_invitations SET expires_at = $1 WHERE id = $2",
		time.Now().Add(-time.Hour), invitationID,
	)
	require.NoError(w.t, err)
	affected, err := result.RowsAffected()
	require.NoError(w.t, err)
	require.EqualValues(w.t, 1, affected)
}

func errorCode(t *testing.T, errs []ct.ApiError) string {
	t.Helper()
	require.NotEmpty(t, errs)
	return errs[0].Code
}

func invitationIDs(items []ct.OrganizationInvitationResponse) []string {
	return functional.Slice(items).Map(func(item ct.OrganizationInvitationResponse) string { return item.Id })
}

func (w world) productUserCount() int {
	w.t.Helper()
	var count int
	require.NoError(w.t, testDB.QueryRow(
		"SELECT COUNT(*) FROM product_users WHERE product_id = $1", w.product.ProductID,
	).Scan(&count))
	return count
}
