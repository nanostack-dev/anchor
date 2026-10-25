package ct_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// allowCustomRoles turns on the organization type's custom_roles switch.
func allowCustomRoles(t *testing.T, product *itdsl.ProductContext, on bool) ct.ProductResourceTypeResponse {
	t.Helper()
	types := product.ResourceTypes()
	organization := types.BySlug("organization")
	return types.Update(organization.Id, ct.UpdateProductResourceTypeJSONRequestBody{CustomRoles: &on})
}

func customRoleBody(name string, permissions ...string) ct.CreateOrganizationRoleJSONRequestBody {
	return ct.CreateOrganizationRoleJSONRequestBody{Name: name, ResourceType: "organization", Permissions: permissions}
}

func TestCustomRolesSwitch(t *testing.T) {
	t.Parallel()
	t.Run("The organization type's custom_roles switch turns on and emits an update", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		sink := product.CaptureEvents()

		updated := allowCustomRoles(t, product, true)

		assert.True(t, updated.CustomRoles)
		assert.True(t, updated.BuiltIn)
		sink.WaitFor("product.resource_type.updated", map[string]string{"resource_type": "organization"})
	})

	t.Run("A product's own type answers 400 to custom_roles", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		tree := declareDocumentTree(t, product)

		resp := product.ResourceTypes().UpdateRaw(tree.project.Id,
			ct.UpdateProductResourceTypeJSONRequestBody{CustomRoles: new(true)})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_NOT_AVAILABLE")
	})

	t.Run("The organization type still refuses a rename with its switch", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		types := product.ResourceTypes()
		organization := types.BySlug("organization")

		resp := types.UpdateRaw(organization.Id,
			ct.UpdateProductResourceTypeJSONRequestBody{CustomRoles: new(true), Name: new("Tenant")})

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "BUILT_IN_RESOURCE_TYPE")
	})
}

func TestCreateCustomRole(t *testing.T) {
	t.Parallel()
	t.Run("With the switch off, a custom role answers 400", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)

		resp := w.product.OrganizationRoles(w.organization.Id).CreateRaw(customRoleBody("Billing auditor"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
	})

	t.Run("With the switch on, a member holding a custom role gets its permissions", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		sink := w.product.CaptureEvents()

		created := w.product.OrganizationRoles(w.organization.Id).
			Create(customRoleBody("Billing auditor", "file:write"))
		external := memberWithExternalID(t, w.product, w.organization.Id, created.Id)

		assert.Equal(t, ct.OrganizationRoleOrigin("custom"), created.Origin)
		assert.Equal(t, "billing auditor", created.Key)
		assert.False(t, created.Customized)
		assert.Nil(t, created.TemplateId)
		assert.Equal(t, []string{"file:write"}, created.Permissions)
		assert.True(t, w.product.Access().User(external).InOrganization(w.organization.Id).
			Can("file:write").Decide().Decision)
		sink.WaitFor("organization.role.created", map[string]string{
			"organization_id": w.organization.Id, "role_id": created.Id,
		})
	})

	t.Run("A custom role without resource_type answers 400", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)

		resp := w.product.OrganizationRoles(w.organization.Id).CreateRaw(
			ct.CreateOrganizationRoleJSONRequestBody{Name: "Billing auditor"})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "RESOURCE_TYPE_REQUIRED")
	})

	t.Run("An unknown permission answers 400", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)

		resp := w.product.OrganizationRoles(w.organization.Id).CreateRaw(customRoleBody("Auditor", "nothing:here"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "PERMISSIONS_NOT_FOUND")
	})
}

func TestCustomRoleKeys(t *testing.T) {
	t.Parallel()
	t.Run("A key a role template holds answers 409", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)

		resp := w.product.OrganizationRoles(w.organization.Id).CreateRaw(customRoleBody("ADMIN"))

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_KEY_DUPLICATE")
	})

	t.Run(
		"A key another custom role of the organization holds answers 409, and another organization may use it",
		func(t *testing.T) {
			t.Parallel()
			w := newRoleTemplateWorld(t)
			allowCustomRoles(t, w.product, true)
			other := w.product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
			w.product.OrganizationRoles(w.organization.Id).Create(customRoleBody("Auditor"))

			again := w.product.OrganizationRoles(w.organization.Id).CreateRaw(customRoleBody("auditor"))
			elsewhere := w.product.OrganizationRoles(other.Id).CreateRaw(customRoleBody("Auditor"))

			requireAPIError(t, again, again.Body, http.StatusConflict, "ROLE_KEY_DUPLICATE")
			assert.Equal(t, http.StatusCreated, elsewhere.StatusCode(), string(elsewhere.Body))
		},
	)

	t.Run("A role template whose key a custom role holds answers 409", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		w.product.OrganizationRoles(w.organization.Id).Create(customRoleBody("Auditor"))

		resp, err := w.product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(t.Context(),
			w.product.ProductID, ct.CreateProductRoleJSONRequestBody{Name: "Auditor", ResourceType: "organization"})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_KEY_DUPLICATE")
	})
}

func TestCustomRoleLimit(t *testing.T) {
	t.Parallel()
	t.Run("An organization's 51st custom role answers 409", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		for i := range 50 {
			roles.Create(customRoleBody(fmt.Sprintf("Custom %02d", i)))
		}

		resp := roles.CreateRaw(customRoleBody("Custom 50"))

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "CUSTOM_ROLE_LIMIT_REACHED")
	})
}

func TestCustomizeCopy(t *testing.T) {
	t.Parallel()
	t.Run(
		"With the switch off, changing a copy answers 400 and choosing it as default still works",
		func(t *testing.T) {
			t.Parallel()
			w := newRoleTemplateWorld(t)
			roles := w.product.OrganizationRoles(w.organization.Id)

			renamed := roles.UpdateRaw(w.admin.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: new("Owner")})
			chosen := roles.Update(w.admin.Id, ct.UpdateOrganizationRoleJSONRequestBody{IsDefault: new(true)})

			requireAPIError(t, renamed, renamed.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
			assert.True(t, chosen.IsDefault)
		},
	)

	t.Run("A customized copy keeps its bundle when the template changes; other copies follow", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		other := w.product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
		roles := w.product.OrganizationRoles(w.organization.Id)
		sink := w.product.CaptureEvents()

		customized := roles.Update(w.admin.Id, ct.UpdateOrganizationRoleJSONRequestBody{
			Permissions: &[]string{"file:read", "file:write"},
		})
		resp, err := w.product.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(t.Context(),
			w.product.ProductID, w.admin.Id, "file:read")
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		assert.True(t, customized.Customized)
		assert.Equal(t, w.admin.Id, customized.Id)
		assert.ElementsMatch(t, []string{"file:read", "file:write"}, roles.Get(w.admin.Id).Permissions)
		assert.Empty(t, w.product.OrganizationRoles(other.Id).Get(w.admin.Id).Permissions)
		sink.WaitFor("organization.role.updated", map[string]string{
			"organization_id": w.organization.Id, "role_id": w.admin.Id,
		})
	})

	t.Run("A reset copy follows its template again", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		roles.Update(w.admin.Id, ct.UpdateOrganizationRoleJSONRequestBody{
			Name: new("Owner"), Permissions: &[]string{"file:write"},
		})

		reset := roles.Reset(w.admin.Id)
		resp, err := w.product.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(t.Context(),
			w.product.ProductID, w.admin.Id, ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: "file:write"})
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		assert.False(t, reset.Customized)
		assert.Equal(t, "Admin", reset.Name)
		assert.Equal(t, []string{"file:read"}, reset.Permissions)
		assert.ElementsMatch(t, []string{"file:read", "file:write"}, roles.Get(w.admin.Id).Permissions)
	})

	t.Run("Resetting a custom role answers 409", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		custom := roles.Create(customRoleBody("Auditor"))

		resp := roles.ResetRaw(custom.Id)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "NOT_A_TEMPLATE_COPY")
	})

	t.Run("Turning the switch off keeps custom roles working and refuses new ones", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		custom := roles.Create(customRoleBody("Auditor", "file:write"))
		external := memberWithExternalID(t, w.product, w.organization.Id, custom.Id)

		allowCustomRoles(t, w.product, false)
		resp := roles.CreateRaw(customRoleBody("Reviewer"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
		assert.True(t, w.product.Access().User(external).InOrganization(w.organization.Id).
			Can("file:write").Decide().Decision)
	})
}

func TestDeleteCustomRole(t *testing.T) {
	t.Parallel()
	t.Run("A custom role no member holds is deleted and emits its event", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		custom := roles.Create(customRoleBody("Auditor"))
		sink := w.product.CaptureEvents()

		resp := roles.DeleteRaw(custom.Id)

		assert.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		read := roles.GetRaw(custom.Id)
		requireAPIError(t, read, read.Body, http.StatusNotFound, "ROLE_NOT_FOUND")
		sink.WaitFor("organization.role.deleted", map[string]string{
			"organization_id": w.organization.Id, "role_id": custom.Id,
		})
	})

	t.Run("A custom role a member holds answers 409", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		custom := roles.Create(customRoleBody("Auditor"))
		addMember(t, w.product, w.organization.Id, createDSLProductUser(t, w.product).ID, custom.Id)

		resp := roles.DeleteRaw(custom.Id)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_IN_USE")
	})

	t.Run("A copy of a template answers 409", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)

		resp := w.product.OrganizationRoles(w.organization.Id).DeleteRaw(w.admin.Id)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "TEMPLATE_COPY_NOT_DELETABLE")
	})
}

func TestDeleteTemplateWithCustomRolesOn(t *testing.T) {
	t.Parallel()
	t.Run("A held copy becomes a custom role, its member keeps access, and unused copies go", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		other := w.product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
		external := memberWithExternalID(t, w.product, w.organization.Id, w.admin.Id)
		sink := w.product.CaptureEvents()

		resp, err := w.product.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(t.Context(),
			w.product.ProductID, w.admin.Id)
		require.NoError(t, err)

		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		assert.True(t, w.product.Access().User(external).InOrganization(w.organization.Id).
			Can("file:read").Decide().Decision)
		listed := w.product.OrganizationRoles(w.organization.Id).Search(ct.SearchOrganizationRolesJSONRequestBody{})
		require.Len(t, listed.Items, 1)
		detached := listed.Items[0]
		assert.Equal(t, ct.OrganizationRoleOrigin("custom"), detached.Origin)
		assert.NotEqual(t, w.admin.Id, detached.Id)
		assert.Empty(t, w.product.OrganizationRoles(other.Id).Search(ct.SearchOrganizationRolesJSONRequestBody{}).Items)
		sink.WaitFor("organization.role.created", map[string]string{
			"organization_id": w.organization.Id, "role_id": detached.Id,
		})
	})

	t.Run("A pending invitation moves onto the detached role and accepts into it", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		user := createDSLProductUser(t, w.product)
		client := w.product.AllScopeAPIKeyClient()
		invited, err := client.CreateOrganizationInvitationWithResponse(t.Context(), w.product.ProductID,
			w.organization.Id, ct.CreateOrganizationInvitationJSONRequestBody{
				Email: openapi_types.Email(user.Email), RoleId: w.admin.Id,
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, invited.StatusCode(), string(invited.Body))

		deleted, err := w.product.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(t.Context(),
			w.product.ProductID, w.admin.Id)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, deleted.StatusCode(), string(deleted.Body))
		accepted, err := client.AcceptOrganizationInvitationWithResponse(
			t.Context(),
			w.product.ProductID,
			w.organization.Id,
			invited.JSON201.Id,
			ct.AcceptOrganizationInvitationJSONRequestBody{ProductUserId: user.ID},
		)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, accepted.StatusCode(), string(accepted.Body))
		listed := w.product.OrganizationRoles(w.organization.Id).Search(ct.SearchOrganizationRolesJSONRequestBody{})
		require.Len(t, listed.Items, 1)
		member, err := client.GetOrganizationMemberWithResponse(t.Context(), w.product.ProductID,
			w.organization.Id, user.ID, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, member.StatusCode(), string(member.Body))
		assert.Equal(t, listed.Items[0].Id, member.JSON200.Role.Id)
	})
}

func TestCustomRolesWithTheSwitchOff(t *testing.T) {
	t.Parallel()
	t.Run("Changing a custom role answers 400, and deleting one or resetting a copy still works", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		custom := roles.Create(customRoleBody("Auditor"))
		roles.Update(w.admin.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: new("Owner")})
		allowCustomRoles(t, w.product, false)

		renamed := roles.UpdateRaw(custom.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: new("Reviewer")})
		reset := roles.Reset(w.admin.Id)
		deleted := roles.DeleteRaw(custom.Id)

		requireAPIError(t, renamed, renamed.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
		assert.False(t, reset.Customized)
		assert.Equal(t, http.StatusNoContent, deleted.StatusCode(), string(deleted.Body))
	})
}

func TestDeleteCustomRoleNamedByAnInvitation(t *testing.T) {
	t.Parallel()
	t.Run("A custom role a pending invitation names answers 409", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		custom := roles.Create(customRoleBody("Auditor"))
		invited, err := w.product.AllScopeAPIKeyClient().CreateOrganizationInvitationWithResponse(t.Context(),
			w.product.ProductID, w.organization.Id, ct.CreateOrganizationInvitationJSONRequestBody{
				Email: openapi_types.Email(uuid.NewString() + "@example.com"), RoleId: custom.Id,
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, invited.StatusCode(), string(invited.Body))

		resp := roles.DeleteRaw(custom.Id)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_IN_USE")
	})
}

func TestDeleteChosenDefaultCustomRole(t *testing.T) {
	t.Parallel()
	t.Run(
		"Deleting the custom role an organization chose as default returns it to the template's copy",
		func(t *testing.T) {
			t.Parallel()
			w := newRoleTemplateWorld(t)
			member := createRoleTemplate(
				t,
				w.product,
				ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
			)
			allowCustomRoles(t, w.product, true)
			roles := w.product.OrganizationRoles(w.organization.Id)
			custom := roles.Create(customRoleBody("Auditor"))
			roles.Update(custom.Id, ct.UpdateOrganizationRoleJSONRequestBody{IsDefault: new(true)})
			sink := w.product.CaptureEvents()

			resp := roles.DeleteRaw(custom.Id)

			require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
			assert.True(t, roles.Get(member.Id).IsDefault)
			sink.WaitFor("organization.role.updated", map[string]string{
				"organization_id": w.organization.Id, "role_id": member.Id,
			})
		},
	)
}

func TestDetachKeepsCustomizedAndDefaultCopies(t *testing.T) {
	t.Parallel()
	t.Run("A customized copy nobody holds becomes a custom role when its template is deleted", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		roles.Update(w.admin.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: new("Owner")})

		resp, err := w.product.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(t.Context(),
			w.product.ProductID, w.admin.Id)
		require.NoError(t, err)

		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		listed := roles.Search(ct.SearchOrganizationRolesJSONRequestBody{})
		require.Len(t, listed.Items, 1)
		assert.Equal(t, "Owner", listed.Items[0].Name)
		assert.Equal(t, ct.OrganizationRoleOrigin("custom"), listed.Items[0].Origin)
	})

	t.Run("The copy of a deleted default template stays the organization's default", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		member := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)
		allowCustomRoles(t, w.product, true)
		addMember(t, w.product, w.organization.Id, createDSLProductUser(t, w.product).ID, member.Id)

		resp, err := w.product.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(t.Context(),
			w.product.ProductID, member.Id)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		added := addMember(t, w.product, w.organization.Id, createDSLProductUser(t, w.product).ID, "")

		detached := w.product.OrganizationRoles(w.organization.Id).Get(added.Role.Id)
		assert.True(t, detached.IsDefault)
		assert.Equal(t, ct.OrganizationRoleOrigin("custom"), detached.Origin)
	})
}

func TestCustomRoleLimitRace(t *testing.T) {
	t.Parallel()
	t.Run("Custom roles created at once never take an organization past 50", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		for i := range 45 {
			roles.Create(customRoleBody(fmt.Sprintf("Custom %02d", i)))
		}

		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range 10 {
			wg.Go(func() {
				<-start
				resp := roles.CreateRaw(customRoleBody(fmt.Sprintf("Racer %02d", i)))
				if resp.StatusCode() != http.StatusCreated {
					requireAPIError(t, resp, resp.Body, http.StatusConflict, "CUSTOM_ROLE_LIMIT_REACHED")
				}
			})
		}
		close(start)
		wg.Wait()

		var custom int
		require.NoError(t, testDB.QueryRowContext(t.Context(), `SELECT count(*) FROM product_roles
			WHERE organization_id = $1 AND template_id IS NULL`, w.organization.Id).Scan(&custom))
		assert.Equal(t, 50, custom)
	})
}

func TestTemplateAndCustomRoleKeyRace(t *testing.T) {
	t.Parallel()
	t.Run("A template and a custom role created with one key at once leave exactly one", func(t *testing.T) {
		t.Parallel()
		const rounds = 10
		for range rounds {
			w := newRoleTemplateWorld(t)
			allowCustomRoles(t, w.product, true)
			key := "auditor-" + uuid.NewString()[:8]

			var templateStatus, customStatus int
			var wg sync.WaitGroup
			start := make(chan struct{})
			wg.Go(func() {
				<-start
				resp, err := w.product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(t.Context(),
					w.product.ProductID, ct.CreateProductRoleJSONRequestBody{
						Name: "Template " + key, Key: &key, ResourceType: "organization",
					})
				if assert.NoError(t, err) {
					templateStatus = resp.StatusCode()
				}
			})
			wg.Go(func() {
				<-start
				body := customRoleBody("Custom " + key)
				body.Key = &key
				customStatus = w.product.OrganizationRoles(w.organization.Id).CreateRaw(body).StatusCode()
			})
			close(start)
			wg.Wait()

			created := 0
			for _, status := range []int{templateStatus, customStatus} {
				if status == http.StatusCreated {
					created++
				} else {
					assert.Equal(t, http.StatusConflict, status)
				}
			}
			assert.Equal(t, 1, created)
		}
	})
}

// Not parallel: it asserts lock waits within tight timeouts. A settled
// invitation, held by the test, pauses the delete after it has counted the
// role's members.
func TestCustomRoleDeleteDuringAMemberAdd(t *testing.T) {
	t.Run("A member added to a custom role while it is deleted never joins", func(t *testing.T) {
		w := newRoleTemplateWorld(t)
		allowCustomRoles(t, w.product, true)
		roles := w.product.OrganizationRoles(w.organization.Id)
		custom := roles.Create(customRoleBody("Auditor"))
		user := createDSLProductUser(t, w.product)
		invitationID := "orginv_" + uuid.NewString()
		_, err := testDB.ExecContext(t.Context(), `INSERT INTO organization_invitations
			(id, product_id, organization_id, email, product_role_id, expires_at)
			VALUES ($1, $2, $3, $4, $5, now() - interval '1 day')`,
			invitationID, w.product.ProductID, w.organization.Id, uuid.NewString()+"@example.com", custom.Id)
		require.NoError(t, err)
		releaseInvitation := holdRowForUpdate(t, "organization_invitations", invitationID)

		deleted := make(chan int, 1)
		go func() { deleted <- roles.DeleteRaw(custom.Id).StatusCode() }()
		waitUntilStatementWaitsOnALock(t, "DELETE", "organization_invitations")
		added := make(chan *ct.AddOrganizationMemberResponse, 1)
		go func() {
			resp, addErr := w.product.AllScopeAPIKeyClient().AddOrganizationMemberWithResponse(context.Background(),
				w.product.ProductID, w.organization.Id,
				ct.AddOrganizationMemberJSONRequestBody{ProductUserId: user.ID, RoleId: custom.Id})
			if assert.NoError(t, addErr) {
				added <- resp
			}
		}()
		waitUntilStatementWaitsOnALock(t, "SELECT", "product_roles")
		releaseInvitation()

		assert.Equal(t, http.StatusNoContent, <-deleted)
		addResp := <-added
		requireAPIError(t, addResp, addResp.Body, http.StatusBadRequest, "ROLE_NOT_FOUND_IN_REQUEST")
	})
}
