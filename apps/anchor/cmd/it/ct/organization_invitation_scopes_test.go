package ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"

	itdsl "anchor/cmd/it/shared/dsl"
)

const (
	scopeCreate = "organization_invitation:create"
	scopeRead   = "organization_invitation:read"
	scopeUpdate = "organization_invitation:update"
	scopeDelete = "organization_invitation:delete"
)

var allInvitationScopes = []string{scopeCreate, scopeRead, scopeUpdate, scopeDelete}

type invitationOperation struct {
	name  string
	scope string
	call  func(w world, client itdsl.InvitationClient, invitation ct.OrganizationInvitationResponse) int
}

func invitationOperations() []invitationOperation {
	return []invitationOperation{
		{
			"create",
			scopeCreate,
			func(w world, c itdsl.InvitationClient, _ ct.OrganizationInvitationResponse) int {
				return c.CreateRaw(w.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
					Email:  openapi_types.Email(uniqueEmail()),
					RoleId: w.roleID,
				}).StatusCode()
			},
		},
		{"search", scopeRead, func(w world, c itdsl.InvitationClient, _ ct.OrganizationInvitationResponse) int {
			return c.SearchRaw(w.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{}).StatusCode()
		}},
		{"get", scopeRead, func(w world, c itdsl.InvitationClient, i ct.OrganizationInvitationResponse) int {
			return c.GetRaw(w.organizationID, i.Id).StatusCode()
		}},
		{"search in product", scopeRead, func(_ world, c itdsl.InvitationClient, _ ct.OrganizationInvitationResponse) int {
			return c.SearchInProductRaw(ct.SearchProductOrganizationInvitationsJSONRequestBody{}).StatusCode()
		}},
		{
			"update",
			scopeUpdate,
			func(w world, c itdsl.InvitationClient, i ct.OrganizationInvitationResponse) int {
				return c.UpdateRaw(w.organizationID, i.Id, ct.UpdateOrganizationInvitationJSONRequestBody{
					RoleId:    w.roleID,
					ExpiresAt: time.Now().Add(time.Hour),
				}).StatusCode()
			},
		},
		{
			"accept",
			scopeUpdate,
			func(w world, c itdsl.InvitationClient, i ct.OrganizationInvitationResponse) int {
				return c.AcceptRaw(w.organizationID, i.Id, w.newProductUser(uniqueEmail())).StatusCode()
			},
		},
		{
			"delete",
			scopeDelete,
			func(w world, c itdsl.InvitationClient, i ct.OrganizationInvitationResponse) int {
				return c.DeleteRaw(w.organizationID, i.Id).StatusCode()
			},
		},
	}
}

func scopesExcept(excluded string) []string {
	var kept []string
	for _, scope := range allInvitationScopes {
		if scope != excluded {
			kept = append(kept, scope)
		}
	}
	return kept
}

func TestInvitationRoutes_RefuseAKeyWithoutTheScopeOfTheOperation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for _, operation := range invitationOperations() {
		t.Run(operation.name, func(t *testing.T) {
			w.t = t
			invitation := w.invite(uniqueEmail())
			client := w.product.Invitations(scopesExcept(operation.scope)...)

			assert.Equal(t, http.StatusForbidden, operation.call(w, client, invitation))
		})
	}
}

func TestInvitationRoutes_AcceptAKeyHoldingOnlyTheScopeOfTheOperation(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	for _, operation := range invitationOperations() {
		t.Run(operation.name, func(t *testing.T) {
			w.t = t
			invitation := w.invite(uniqueEmail())
			client := w.product.Invitations(operation.scope)

			assert.Less(t, operation.call(w, client, invitation), http.StatusMultipleChoices)
		})
	}
}
