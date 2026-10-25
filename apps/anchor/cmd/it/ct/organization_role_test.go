package ct_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// roleTemplateWorld is a product with a "file:read" permission, an "admin"
// template bundling it, and one organization.
type roleTemplateWorld struct {
	product      *itdsl.ProductContext
	admin        ct.ProductRoleResponse
	organization ct.ProductOrganizationResponse
}

func newRoleTemplateWorld(t *testing.T) roleTemplateWorld {
	t.Helper()
	product := createTestProductContext(t)
	product.CreateProductResourcePermissions(t, "file:read", "file:write")
	return roleTemplateWorld{
		product: product,
		admin: createRoleTemplate(
			t,
			product,
			ct.CreateProductRoleJSONRequestBody{Name: "Admin", Permissions: []string{"file:read"}},
		),
		organization: product.CreateOrganization(t, "Org-"+uuid.NewString(), nil),
	}
}

func createRoleTemplate(
	t *testing.T,
	product *itdsl.ProductContext,
	body ct.CreateProductRoleJSONRequestBody,
) ct.ProductRoleResponse {
	t.Helper()
	if body.ResourceType == "" {
		body.ResourceType = "organization"
	}
	resp, err := product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(t.Context(), product.ProductID, body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return *resp.JSON201
}

func addMemberRaw(
	t *testing.T, product *itdsl.ProductContext, organizationID, productUserID, roleID string,
) *ct.AddOrganizationMemberResponse {
	t.Helper()
	resp, err := product.AllScopeAPIKeyClient().AddOrganizationMemberWithResponse(t.Context(), product.ProductID,
		organizationID, ct.AddOrganizationMemberJSONRequestBody{ProductUserId: productUserID, RoleId: roleID})
	require.NoError(t, err)
	return resp
}

func addMember(
	t *testing.T,
	product *itdsl.ProductContext,
	organizationID, productUserID, roleID string,
) ct.OrganizationMemberResponse {
	t.Helper()
	resp := addMemberRaw(t, product, organizationID, productUserID, roleID)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return *resp.JSON201
}

func TestOrganizationRolesAreCopiesOfTemplates(t *testing.T) {
	t.Parallel()
	t.Run("An organization lists its own copy of every template, under the template's ID and key", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)

		found := w.product.OrganizationRoles(w.organization.Id).Search(ct.SearchOrganizationRolesJSONRequestBody{})

		require.Len(t, found.Items, 1)
		copied := found.Items[0]
		assert.Equal(t, w.admin.Id, copied.Id)
		assert.Equal(t, w.admin.Id, *copied.TemplateId)
		assert.Equal(t, w.organization.Id, copied.OrganizationId)
		assert.Equal(t, "admin", copied.Key)
		assert.Equal(t, "Admin", copied.Name)
		assert.Equal(t, ct.OrganizationRoleOrigin("template"), copied.Origin)
		assert.Equal(t, "organization", copied.ResourceType)
		assert.Equal(t, []string{"file:read"}, copied.Permissions)
		assert.False(t, copied.IsDefault)
	})
}

func TestOrganizationRolesOfAnotherOrganizationAreSeparate(t *testing.T) {
	t.Parallel()
	t.Run("A member is held by a copy of the template, stored apart from the template", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		user := createDSLProductUser(t, w.product)

		member := addMember(t, w.product, w.organization.Id, user.ID, w.admin.Id)

		assert.Equal(t, w.admin.Id, member.Role.Id)
		var heldRow, heldTemplate string
		require.NoError(t, testDB.QueryRowContext(t.Context(), `
			SELECT r.id, r.template_id FROM organization_memberships m
			JOIN product_roles r ON r.id = m.product_role_id
			WHERE m.organization_id = $1 AND m.product_user_id = $2`, w.organization.Id, user.ID,
		).Scan(&heldRow, &heldTemplate))
		assert.NotEqual(t, w.admin.Id, heldRow)
		assert.Equal(t, w.admin.Id, heldTemplate)
	})
}

func TestTemplateChangeReachesEveryOrganization(t *testing.T) {
	t.Parallel()
	t.Run("Adding a permission to a template grants it to members of every organization", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		other := w.product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
		firstExternal := memberWithExternalID(t, w.product, w.organization.Id, w.admin.Id)
		secondExternal := memberWithExternalID(t, w.product, other.Id, w.admin.Id)
		before := w.product.Access().User(firstExternal).InOrganization(w.organization.Id).Can("file:write").Decide()
		require.False(t, before.Decision)

		resp, err := w.product.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(t.Context(),
			w.product.ProductID, w.admin.Id, ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: "file:write"})
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		assert.True(
			t,
			w.product.Access().
				User(firstExternal).
				InOrganization(w.organization.Id).
				Can("file:write").
				Decide().
				Decision,
		)
		assert.True(
			t,
			w.product.Access().User(secondExternal).InOrganization(other.Id).Can("file:write").Decide().Decision,
		)
		copied := w.product.OrganizationRoles(other.Id).Get(w.admin.Id)
		assert.ElementsMatch(t, []string{"file:read", "file:write"}, copied.Permissions)
	})
}

// memberWithExternalID adds a member the access evaluation can name, and
// returns that external ID.
func memberWithExternalID(t *testing.T, product *itdsl.ProductContext, organizationID, roleID string) string {
	t.Helper()
	user := createDSLProductUser(t, product)
	externalID := "user_" + uuid.NewString()
	setDSLProductUserExternalID(t, product, user.ID, externalID)
	addMember(t, product, organizationID, user.ID, roleID)
	return externalID
}

func TestTemplateRenameReachesCopies(t *testing.T) {
	t.Parallel()
	t.Run("Renaming a template renames its copies and keeps their key", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)

		resp, err := w.product.OwnerAuthenticatedClient().
			UpdateProductRoleWithResponse(t.Context(), w.product.ProductID,
				w.admin.Id, ct.UpdateProductRoleJSONRequestBody{Name: "Administrator"})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))

		copied := w.product.OrganizationRoles(w.organization.Id).Get(w.admin.Id)
		assert.Equal(t, "Administrator", copied.Name)
		assert.Equal(t, "admin", copied.Key)
	})
}

func TestCreateTemplateKey(t *testing.T) {
	t.Parallel()
	t.Run("A template's key defaults to its name in lower case, or is the key it names", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		derived := createRoleTemplate(t, product, ct.CreateProductRoleJSONRequestBody{Name: "Billing Auditor"})
		named := createRoleTemplate(
			t,
			product,
			ct.CreateProductRoleJSONRequestBody{Name: "Project admin", Key: new("project-admin")},
		)

		assert.Equal(t, "billing auditor", derived.Key)
		assert.Equal(t, "project-admin", named.Key)
	})
}

func TestCreateTemplateDuplicateKey(t *testing.T) {
	t.Parallel()
	t.Run("A key another template holds, in any case, answers 409", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		createRoleTemplate(t, product, ct.CreateProductRoleJSONRequestBody{Name: "Admin", Key: new("admin")})

		resp, err := product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(t.Context(), product.ProductID,
			ct.CreateProductRoleJSONRequestBody{Name: "Owner", Key: new("ADMIN"), ResourceType: "organization"})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_KEY_DUPLICATE")
	})
}

func TestDefaultTemplate(t *testing.T) {
	t.Parallel()
	t.Run("Marking a template default moves the default away from the previous one", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		member := createRoleTemplate(
			t,
			product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)

		viewer := createRoleTemplate(
			t,
			product,
			ct.CreateProductRoleJSONRequestBody{Name: "Viewer", IsDefault: new(true)},
		)

		read, err := product.OwnerAuthenticatedClient().
			GetProductRoleWithResponse(t.Context(), product.ProductID, member.Id)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, read.StatusCode())
		assert.True(t, viewer.IsDefault)
		assert.False(t, read.JSON200.IsDefault)
	})
}

func TestDefaultTemplateMustBeOrganizationScoped(t *testing.T) {
	t.Parallel()
	t.Run("Marking a project template default answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		declareDocumentTree(t, product)

		resp, err := product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(t.Context(), product.ProductID,
			ct.CreateProductRoleJSONRequestBody{Name: "Project member", ResourceType: "project", IsDefault: new(true)})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "DEFAULT_ROLE_NOT_ORGANIZATION_SCOPED")
	})
}

func TestAddMemberWithoutRoleUsesDefault(t *testing.T) {
	t.Parallel()
	t.Run("A member added with no role gets the organization's copy of the default template", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		member := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)
		user := createDSLProductUser(t, w.product)

		added := addMember(t, w.product, w.organization.Id, user.ID, "")

		assert.Equal(t, member.Id, added.Role.Id)
	})
}

func TestAddMemberWithoutRoleOrDefault(t *testing.T) {
	t.Parallel()
	t.Run("A member added with no role answers 400 while there is no default role", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		user := createDSLProductUser(t, w.product)

		resp := addMemberRaw(t, w.product, w.organization.Id, user.ID, "")

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_REQUIRED")
	})
}

func TestOrganizationChoosesItsDefaultRole(t *testing.T) {
	t.Parallel()
	t.Run("An organization's own default wins over the product's default template", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		sink := w.product.CaptureEvents()
		member := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)
		roles := w.product.OrganizationRoles(w.organization.Id)

		chosen := roles.Update(w.admin.Id, ct.UpdateOrganizationRoleJSONRequestBody{IsDefault: new(true)})
		user := createDSLProductUser(t, w.product)
		added := addMember(t, w.product, w.organization.Id, user.ID, "")

		assert.True(t, chosen.IsDefault)
		assert.Equal(t, w.admin.Id, added.Role.Id)
		assert.False(t, roles.Get(member.Id).IsDefault)
		sink.WaitFor("organization.role.updated", map[string]string{
			"organization_id": w.organization.Id, "role_id": w.admin.Id,
		})
		sink.WaitFor("organization.role.updated", map[string]string{
			"organization_id": w.organization.Id, "role_id": member.Id,
		})
	})
}

func TestOrganizationDefaultRoleMustBeOrganizationScoped(t *testing.T) {
	t.Parallel()
	t.Run("Choosing a project role as the organization's default answers 400", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		declareDocumentTree(t, w.product)
		project := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Project lead", ResourceType: "project"},
		)

		resp := w.product.OrganizationRoles(w.organization.Id).UpdateRaw(project.Id,
			ct.UpdateOrganizationRoleJSONRequestBody{IsDefault: new(true)})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "DEFAULT_ROLE_NOT_ORGANIZATION_SCOPED")
	})
}

func TestGetOrganizationRoleNotFound(t *testing.T) {
	t.Parallel()
	t.Run("A role of another product answers 404", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		elsewhere := newRoleTemplateWorld(t)

		resp := w.product.OrganizationRoles(w.organization.Id).GetRaw(elsewhere.admin.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "ROLE_NOT_FOUND")
	})
}

func TestInvitationWithoutRoleUsesDefault(t *testing.T) {
	t.Parallel()
	t.Run("An invitation with no role names the default template, and accepting it holds the copy", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		member := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)
		user := createDSLProductUser(t, w.product)
		client := w.product.AllScopeAPIKeyClient()

		created, err := client.CreateOrganizationInvitationWithResponse(
			t.Context(),
			w.product.ProductID,
			w.organization.Id,
			ct.CreateOrganizationInvitationJSONRequestBody{Email: openapi_types.Email(user.Email)},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))
		accepted, err := client.AcceptOrganizationInvitationWithResponse(
			t.Context(),
			w.product.ProductID,
			w.organization.Id,
			created.JSON201.Id,
			ct.AcceptOrganizationInvitationJSONRequestBody{ProductUserId: user.ID},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, accepted.StatusCode(), string(accepted.Body))

		assert.Equal(t, member.Id, created.JSON201.RoleId)
		found, err := client.GetOrganizationMemberWithResponse(
			t.Context(),
			w.product.ProductID,
			w.organization.Id,
			user.ID,
			nil,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, found.StatusCode())
		assert.Equal(t, member.Id, found.JSON200.Role.Id)
	})
}

func TestDeleteUnassignedTemplateDeletesCopies(t *testing.T) {
	t.Parallel()
	t.Run("Deleting a template no member holds removes every organization's copy", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		w.product.OrganizationRoles(w.organization.Id).Get(w.admin.Id)

		resp, err := w.product.OwnerAuthenticatedClient().
			DeleteProductRoleWithResponse(t.Context(), w.product.ProductID, w.admin.Id)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		found := w.product.OrganizationRoles(w.organization.Id).Search(ct.SearchOrganizationRolesJSONRequestBody{})
		assert.Empty(t, found.Items)
	})
}

func TestDeleteTemplateHeldInAnOrganization(t *testing.T) {
	t.Parallel()
	t.Run("Deleting a template a member holds through a copy answers 409", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		user := createDSLProductUser(t, w.product)
		addMember(t, w.product, w.organization.Id, user.ID, w.admin.Id)

		resp, err := w.product.OwnerAuthenticatedClient().
			DeleteProductRoleWithResponse(t.Context(), w.product.ProductID, w.admin.Id)
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_IN_USE")
	})
}

func TestSearchMembersByTemplateID(t *testing.T) {
	t.Parallel()
	t.Run("Filtering members by a template's ID finds the members holding its copy", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		user := createDSLProductUser(t, w.product)
		addMember(t, w.product, w.organization.Id, user.ID, w.admin.Id)

		resp, err := w.product.AllScopeAPIKeyClient().SearchOrganizationMembersWithResponse(t.Context(),
			w.product.ProductID, w.organization.Id, ct.SearchOrganizationMembersJSONRequestBody{
				Filter: &ct.OrganizationMemberFilter{RoleIds: &[]string{w.admin.Id}},
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))

		ids := functional.Slice(resp.JSON200.Items).
			Map(func(m ct.OrganizationMemberResponse) string { return m.ProductUserId })
		assert.Equal(t, []string{user.ID}, []string(ids))
	})
}

func TestOrganizationShowsTheDefaultTemplateCopyAsDefault(t *testing.T) {
	t.Parallel()
	t.Run(
		"With no choice of its own, an organization's copy of the default template is its default",
		func(t *testing.T) {
			t.Parallel()
			w := newRoleTemplateWorld(t)
			member := createRoleTemplate(
				t,
				w.product,
				ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
			)
			roles := w.product.OrganizationRoles(w.organization.Id)

			listed := roles.Search(ct.SearchOrganizationRolesJSONRequestBody{})

			assert.True(t, roles.Get(member.Id).IsDefault)
			defaults := functional.Slice(listed.Items).FilterMap(
				func(r ct.OrganizationRoleResponse) bool { return r.IsDefault },
				func(r ct.OrganizationRoleResponse) string { return r.Id },
			)
			assert.Equal(t, []string{member.Id}, []string(defaults))
		},
	)
}

func TestTemplateRevokeReachesEveryOrganization(t *testing.T) {
	t.Parallel()
	t.Run("Removing a permission from a template takes it from members of every organization", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		other := w.product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
		external := memberWithExternalID(t, w.product, other.Id, w.admin.Id)

		resp, err := w.product.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(t.Context(),
			w.product.ProductID, w.admin.Id, "file:read")
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		assert.False(
			t,
			w.product.Access().User(external).InOrganization(other.Id).Can("file:read").Decide().Decision,
		)
		assert.Empty(t, w.product.OrganizationRoles(other.Id).Get(w.admin.Id).Permissions)
	})
}

// Not parallel within its rounds: each round races a copy being made against
// a change of its template.
func TestCopyMadeDuringATemplateChangeHoldsTheChange(t *testing.T) {
	t.Parallel()
	t.Run("A copy made while a permission leaves its template never keeps that permission", func(t *testing.T) {
		t.Parallel()
		const rounds = 20
		for range rounds {
			product := createTestProductContext(t)
			product.CreateProductResourcePermissions(t, "file:read")
			organization := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
			template := createRoleTemplate(t, product,
				ct.CreateProductRoleJSONRequestBody{Name: "Reader", Permissions: []string{"file:read"}})
			user := createDSLProductUser(t, product)
			client := product.AllScopeAPIKeyClient()

			var wg sync.WaitGroup
			start := make(chan struct{})
			wg.Go(func() {
				<-start
				resp, err := client.AddOrganizationMemberWithResponse(
					t.Context(),
					product.ProductID,
					organization.Id,
					ct.AddOrganizationMemberJSONRequestBody{ProductUserId: user.ID, RoleId: template.Id},
				)
				if assert.NoError(t, err) {
					assert.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
				}
			})
			wg.Go(func() {
				<-start
				resp, err := product.OwnerAuthenticatedClient().UnassignPermissionFromProductRoleWithResponse(
					t.Context(), product.ProductID, template.Id, "file:read")
				if assert.NoError(t, err) {
					assert.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
				}
			})
			close(start)
			wg.Wait()

			assert.Empty(t, product.OrganizationRoles(organization.Id).Get(template.Id).Permissions)
		}
	})
}

func TestConcurrentDefaultChangesLeaveOneDefault(t *testing.T) {
	t.Parallel()
	t.Run("Two roles chosen as default at once leave exactly one, and a refused one answers 409", func(t *testing.T) {
		t.Parallel()
		const rounds = 10
		for range rounds {
			w := newRoleTemplateWorld(t)
			other := createRoleTemplate(t, w.product, ct.CreateProductRoleJSONRequestBody{Name: "Member"})
			roles := w.product.OrganizationRoles(w.organization.Id)
			roles.Get(w.admin.Id)

			var wg sync.WaitGroup
			start := make(chan struct{})
			for _, roleID := range []string{w.admin.Id, other.Id} {
				wg.Go(func() {
					<-start
					resp := roles.UpdateRaw(roleID, ct.UpdateOrganizationRoleJSONRequestBody{IsDefault: new(true)})
					if resp.StatusCode() != http.StatusOK {
						requireAPIError(t, resp, resp.Body, http.StatusConflict, "DEFAULT_ROLE_CONFLICT")
					}
				})
			}
			close(start)
			wg.Wait()

			listed := roles.Search(ct.SearchOrganizationRolesJSONRequestBody{})
			defaults := functional.Slice(listed.Items).Filter(
				func(r ct.OrganizationRoleResponse) bool { return r.IsDefault },
			)
			assert.Len(t, defaults, 1)
		}
	})
}

func TestOrganizationRoleOfAnotherOrganizationIsRefused(t *testing.T) {
	t.Parallel()
	t.Run("A copy's own ID from another organization names no role here", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		other := w.product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
		var otherCopyID string
		require.NoError(t, testDB.QueryRowContext(t.Context(), `SELECT id FROM product_roles
			WHERE organization_id = $1 AND template_id = $2`, other.Id, w.admin.Id).Scan(&otherCopyID))
		user := createDSLProductUser(t, w.product)

		read := w.product.OrganizationRoles(w.organization.Id).GetRaw(otherCopyID)
		added := addMemberRaw(t, w.product, w.organization.Id, user.ID, otherCopyID)

		requireAPIError(t, read, read.Body, http.StatusNotFound, "ROLE_NOT_FOUND")
		requireAPIError(t, added, added.Body, http.StatusBadRequest, "ROLE_NOT_FOUND_IN_REQUEST")
	})
}

func TestInvitationWithoutRoleOrDefault(t *testing.T) {
	t.Parallel()
	t.Run("An invitation with no role answers 400 while there is no default role", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)

		resp, err := w.product.AllScopeAPIKeyClient().CreateOrganizationInvitationWithResponse(
			t.Context(), w.product.ProductID, w.organization.Id,
			ct.CreateOrganizationInvitationJSONRequestBody{
				Email: openapi_types.Email(uuid.NewString() + "@example.com"),
			},
		)
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_REQUIRED")
	})
}

func TestFoundingMemberWithoutRole(t *testing.T) {
	t.Parallel()
	t.Run("A founding member with no role gets the copy of the default template", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		member := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)
		user := createDSLProductUser(t, w.product)

		resp := createOrganizationWithFounder(t, w.product, user.ID)

		require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
		found, err := w.product.AllScopeAPIKeyClient().GetOrganizationMemberWithResponse(
			t.Context(), w.product.ProductID, resp.JSON201.Id, user.ID, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, found.StatusCode(), string(found.Body))
		assert.Equal(t, member.Id, found.JSON200.Role.Id)
		var held string
		require.NoError(t, testDB.QueryRowContext(t.Context(), `SELECT r.template_id FROM organization_memberships m
			JOIN product_roles r ON r.id = m.product_role_id
			WHERE m.organization_id = $1 AND m.product_user_id = $2`, resp.JSON201.Id, user.ID).Scan(&held))
		assert.Equal(t, member.Id, held)
	})

	t.Run("A founding member with no role answers 400 while there is no default role", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		user := createDSLProductUser(t, w.product)

		resp := createOrganizationWithFounder(t, w.product, user.ID)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_REQUIRED")
	})
}

func createOrganizationWithFounder(
	t *testing.T, product *itdsl.ProductContext, productUserID string,
) *ct.CreateProductOrganizationResponse {
	t.Helper()
	resp, err := product.AllScopeAPIKeyClient().CreateProductOrganizationWithResponse(
		t.Context(), product.ProductID, ct.CreateProductOrganizationJSONRequestBody{
			Name:           "Org-" + uuid.NewString(),
			FoundingMember: &ct.FoundingMemberRequest{ProductUserId: productUserID},
		})
	require.NoError(t, err)
	return resp
}

func TestOrganizationRoleSearchLimits(t *testing.T) {
	t.Parallel()
	over := func(n int, value func(int) string) []string {
		items := make([]string, n)
		for i := range items {
			items[i] = value(i)
		}
		return items
	}
	cases := []struct {
		name   string
		filter ct.OrganizationRoleFilter
	}{
		{"More than 100 IDs answers 400", ct.OrganizationRoleFilter{
			Ids: over(101, func(int) string { return "product_role_" + uuid.NewString() }),
		}},
		{"More than 100 keys answers 400", ct.OrganizationRoleFilter{
			Keys: over(101, func(int) string { return "key-" + uuid.NewString() }),
		}},
		{"More than 50 resource types answers 400", ct.OrganizationRoleFilter{
			ResourceTypes: over(51, func(int) string { return "organization" }),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			w := newRoleTemplateWorld(t)

			resp := w.product.OrganizationRoles(w.organization.Id).SearchRaw(
				ct.SearchOrganizationRolesJSONRequestBody{Filter: &tc.filter})

			requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
		})
	}
}

func TestCreateTemplateKeyTooLong(t *testing.T) {
	t.Parallel()
	t.Run("A key longer than 100 characters answers 400", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		key := strings.Repeat("k", 101)

		resp, err := product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(t.Context(), product.ProductID,
			ct.CreateProductRoleJSONRequestBody{Name: "Admin", ResourceType: "organization", Key: &key})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})
}

func updateTemplate(
	t *testing.T, product *itdsl.ProductContext, id string, body ct.UpdateProductRoleJSONRequestBody,
) *ct.UpdateProductRoleResponse {
	t.Helper()
	resp, err := product.OwnerAuthenticatedClient().
		UpdateProductRoleWithResponse(t.Context(), product.ProductID, id, body)
	require.NoError(t, err)
	return resp
}

func getTemplate(t *testing.T, product *itdsl.ProductContext, id string) ct.ProductRoleResponse {
	t.Helper()
	resp, err := product.OwnerAuthenticatedClient().GetProductRoleWithResponse(t.Context(), product.ProductID, id)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	return *resp.JSON200
}

func TestRenameKeepsTheDefaultTemplate(t *testing.T) {
	t.Parallel()
	t.Run("Renaming the default template keeps it the default", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		member := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)

		resp := updateTemplate(t, w.product, member.Id, ct.UpdateProductRoleJSONRequestBody{Name: "Member renamed"})

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.True(t, getTemplate(t, w.product, member.Id).IsDefault)
	})
}

func TestUnmarkDefaultTemplate(t *testing.T) {
	t.Parallel()
	t.Run("Unmarking the default template leaves the product with no default role", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		member := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)
		user := createDSLProductUser(t, w.product)

		resp := updateTemplate(t, w.product, member.Id,
			ct.UpdateProductRoleJSONRequestBody{Name: member.Name, IsDefault: new(false)})

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.False(t, resp.JSON200.IsDefault)
		added := addMemberRaw(t, w.product, w.organization.Id, user.ID, "")
		requireAPIError(t, added, added.Body, http.StatusBadRequest, "ROLE_REQUIRED")
	})
}

func TestMovingTheDefaultTemplateReportsTheOldOne(t *testing.T) {
	t.Parallel()
	t.Run("Marking another template default emits an update for the template that lost it", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		member := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)
		sink := w.product.CaptureEvents()

		resp := updateTemplate(t, w.product, w.admin.Id,
			ct.UpdateProductRoleJSONRequestBody{Name: w.admin.Name, IsDefault: new(true)})

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.False(t, getTemplate(t, w.product, member.Id).IsDefault)
		sink.WaitFor("product.role.updated", map[string]string{"role_id": member.Id})
	})
}

func TestConcurrentTemplateDefaultsLeaveOneDefault(t *testing.T) {
	t.Parallel()
	t.Run("Two templates marked default at once leave exactly one, and a refused one answers 409", func(t *testing.T) {
		t.Parallel()
		const rounds = 10
		for range rounds {
			w := newRoleTemplateWorld(t)
			other := createRoleTemplate(t, w.product, ct.CreateProductRoleJSONRequestBody{Name: "Member"})

			var wg sync.WaitGroup
			start := make(chan struct{})
			for _, template := range []ct.ProductRoleResponse{w.admin, other} {
				wg.Go(func() {
					<-start
					resp := updateTemplate(t, w.product, template.Id,
						ct.UpdateProductRoleJSONRequestBody{Name: template.Name, IsDefault: new(true)})
					if resp.StatusCode() != http.StatusOK {
						requireAPIError(t, resp, resp.Body, http.StatusConflict, "DEFAULT_ROLE_CONFLICT")
					}
				})
			}
			close(start)
			wg.Wait()

			defaults := 0
			for _, id := range []string{w.admin.Id, other.Id} {
				if getTemplate(t, w.product, id).IsDefault {
					defaults++
				}
			}
			assert.Equal(t, 1, defaults)
		}
	})
}

func TestDefaultMoveDuringOrganizationCreate(t *testing.T) {
	t.Parallel()
	t.Run(
		"Moving the default template while an organization copies its templates answers no error",
		func(t *testing.T) {
			t.Parallel()
			const rounds = 10
			for range rounds {
				w := newRoleTemplateWorld(t)
				createRoleTemplate(
					t,
					w.product,
					ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
				)
				client := w.product.AllScopeAPIKeyClient()

				var wg sync.WaitGroup
				start := make(chan struct{})
				wg.Go(func() {
					<-start
					resp := updateTemplate(t, w.product, w.admin.Id,
						ct.UpdateProductRoleJSONRequestBody{Name: w.admin.Name, IsDefault: new(true)})
					assert.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
				})
				wg.Go(func() {
					<-start
					resp, err := client.CreateProductOrganizationWithResponse(t.Context(), w.product.ProductID,
						ct.CreateProductOrganizationJSONRequestBody{Name: "Org-" + uuid.NewString()})
					if assert.NoError(t, err) {
						assert.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
					}
				})
				close(start)
				wg.Wait()
			}
		},
	)
}

// Not parallel: it asserts lock waits within tight timeouts. A settled
// invitation, held by the test, pauses the delete after it has counted the
// template's members.
func TestTemplateDeleteDuringAMemberAdd(t *testing.T) {
	t.Run("A member added on a copy while its template is deleted never joins", func(t *testing.T) {
		product := createTestProductContext(t)
		template := createRoleTemplate(t, product, ct.CreateProductRoleJSONRequestBody{Name: "Reader"})
		organization := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
		user := createDSLProductUser(t, product)
		client := product.AllScopeAPIKeyClient()
		invitationID := "orginv_" + uuid.NewString()
		_, err := testDB.ExecContext(t.Context(), `INSERT INTO organization_invitations
			(id, product_id, organization_id, email, product_role_id, expires_at)
			VALUES ($1, $2, $3, $4, $5, now() - interval '1 day')`,
			invitationID, product.ProductID, organization.Id, uuid.NewString()+"@example.com", template.Id)
		require.NoError(t, err)
		releaseInvitation := holdRowForUpdate(t, "organization_invitations", invitationID)

		deleted := make(chan int, 1)
		go func() {
			resp, deleteErr := product.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				context.Background(), product.ProductID, template.Id)
			if assert.NoError(t, deleteErr) {
				deleted <- resp.StatusCode()
			}
		}()
		waitUntilStatementWaitsOnALock(t, "DELETE", "organization_invitations")
		added := make(chan *ct.AddOrganizationMemberResponse, 1)
		go func() {
			resp, addErr := client.AddOrganizationMemberWithResponse(context.Background(), product.ProductID,
				organization.Id, ct.AddOrganizationMemberJSONRequestBody{ProductUserId: user.ID, RoleId: template.Id})
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

func TestOrganizationUnchoosesItsDefault(t *testing.T) {
	t.Parallel()
	t.Run("Unchoosing its own default returns the organization to the default template's copy", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		member := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)
		roles := w.product.OrganizationRoles(w.organization.Id)
		roles.Update(w.admin.Id, ct.UpdateOrganizationRoleJSONRequestBody{IsDefault: new(true)})
		sink := w.product.CaptureEvents()

		unchosen := roles.Update(w.admin.Id, ct.UpdateOrganizationRoleJSONRequestBody{IsDefault: new(false)})

		assert.False(t, unchosen.IsDefault)
		assert.True(t, roles.Get(member.Id).IsDefault)
		sink.WaitFor("organization.role.updated", map[string]string{
			"organization_id": w.organization.Id, "role_id": member.Id,
		})
	})

	t.Run("Unchoosing the default template's copy changes nothing", func(t *testing.T) {
		t.Parallel()
		w := newRoleTemplateWorld(t)
		member := createRoleTemplate(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
		)
		roles := w.product.OrganizationRoles(w.organization.Id)

		unchosen := roles.Update(member.Id, ct.UpdateOrganizationRoleJSONRequestBody{IsDefault: new(false)})

		assert.True(t, unchosen.IsDefault)
	})

	t.Run(
		"Choosing the default template's copy keeps the organization following the product's default",
		func(t *testing.T) {
			t.Parallel()
			w := newRoleTemplateWorld(t)
			member := createRoleTemplate(
				t,
				w.product,
				ct.CreateProductRoleJSONRequestBody{Name: "Member", IsDefault: new(true)},
			)
			roles := w.product.OrganizationRoles(w.organization.Id)
			roles.Update(member.Id, ct.UpdateOrganizationRoleJSONRequestBody{IsDefault: new(true)})

			resp := updateTemplate(t, w.product, w.admin.Id,
				ct.UpdateProductRoleJSONRequestBody{Name: w.admin.Name, IsDefault: new(true)})

			require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
			assert.True(t, roles.Get(w.admin.Id).IsDefault)
			assert.False(t, roles.Get(member.Id).IsDefault)
		},
	)
}
