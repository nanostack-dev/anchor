package ct_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// switchCustomRoles sets the allows_custom_roles switch of one resource type.
func switchCustomRoles(t *testing.T, product *itdsl.ProductContext, slug string, on bool) {
	t.Helper()
	types := product.ResourceTypes()
	types.Update(types.BySlug(slug).Id, ct.UpdateProductResourceTypeJSONRequestBody{AllowsCustomRoles: &on})
}

func scopedCustomRoleBody(name, resourceType string, permissions ...string) ct.CreateOrganizationRoleJSONRequestBody {
	return ct.CreateOrganizationRoleJSONRequestBody{Name: name, ResourceType: resourceType, Permissions: permissions}
}

// projectRoleWorld is a product with the document tree, one permission scoped
// to each of its types, and one organization.
type projectRoleWorld struct {
	product      *itdsl.ProductContext
	client       *ct.ClientWithResponses
	organization ct.ProductOrganizationResponse
}

func newProjectRoleWorld(t *testing.T) projectRoleWorld {
	t.Helper()
	product := createTestProductContext(t)
	declareDocumentTree(t, product)
	product.CreateProductResourcePermissions(t, "members:update")
	product.CreateScopedPermissions(t, "project", "project:manage", "project:archive")
	product.CreateScopedPermissions(t, "folder", "folder:edit")
	product.CreateScopedPermissions(t, "document", "document:view")
	return projectRoleWorld{
		product:      product,
		client:       product.AllScopeAPIKeyClient(),
		organization: product.CreateOrganization(t, "Org-"+uuid.NewString(), nil),
	}
}

func (w projectRoleWorld) createRaw(
	t *testing.T, body ct.CreateOrganizationRoleJSONRequestBody,
) *ct.CreateOrganizationRoleResponse {
	t.Helper()
	resp, err := w.client.CreateOrganizationRoleWithResponse(
		context.Background(), w.product.ProductID, w.organization.Id, body)
	require.NoError(t, err)
	return resp
}

func (w projectRoleWorld) create(t *testing.T, body ct.CreateOrganizationRoleJSONRequestBody) ct.ProductRoleResponse {
	t.Helper()
	resp := w.createRaw(t, body)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return *resp.JSON201
}

func (w projectRoleWorld) updateRaw(
	t *testing.T, roleID string, body ct.UpdateOrganizationRoleJSONRequestBody,
) *ct.UpdateOrganizationRoleResponse {
	t.Helper()
	resp, err := w.client.UpdateOrganizationRoleWithResponse(
		context.Background(), w.product.ProductID, w.organization.Id, roleID, body)
	require.NoError(t, err)
	return resp
}

func (w projectRoleWorld) get(t *testing.T, roleID string) ct.ProductRoleResponse {
	t.Helper()
	resp, err := w.client.GetOrganizationRoleWithResponse(
		context.Background(), w.product.ProductID, w.organization.Id, roleID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	return *resp.JSON200
}

func grantNames(role ct.ProductRoleResponse) []string {
	names := make([]string, 0, len(role.Permissions))
	for _, grant := range role.Permissions {
		names = append(names, grant.PermissionName)
	}
	return names
}

func TestCustomRolesSwitchPerType(t *testing.T) {
	t.Parallel()
	t.Run("Each type keeps its own switch: project on leaves folder and organization off", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)

		switchCustomRoles(t, w.product, "project", true)

		types := w.product.ResourceTypes()
		assert.True(t, types.BySlug("project").AllowsCustomRoles)
		assert.False(t, types.BySlug("folder").AllowsCustomRoles)
		assert.False(t, types.BySlug("organization").AllowsCustomRoles)
	})

	t.Run("Publishing the tree, which carries no switch, keeps a type's switch on", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		product.ResourceTypes().CreateUnder("project", "organization")
		switchCustomRoles(t, product, "project", true)

		published := product.ResourceTypes().ReplaceTree(ct.ProductResourceTypeTreeItem{
			Slug: "project", Name: "Projects", ParentTypes: []string{"organization"},
		})

		require.Len(t, published, 2)
		assert.Equal(t, "Projects", published[1].Name)
		assert.True(t, published[1].AllowsCustomRoles)
	})

	t.Run("With the organization switch on and project off, a project custom role answers 400", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "organization", true)

		resp := w.createRaw(t, scopedCustomRoleBody("Release manager", "project", "project:manage"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
	})

	t.Run(
		"With the project switch on and organization off, an organization custom role answers 400",
		func(t *testing.T) {
			t.Parallel()
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "project", true)

			resp := w.createRaw(t, scopedCustomRoleBody("Billing auditor", "organization", "members:update"))

			requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
		},
	)

	t.Run("With the folder switch on, a project custom role still answers 400", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "folder", true)

		resp := w.createRaw(t, scopedCustomRoleBody("Release manager", "project", "project:manage"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
	})
}

func TestCreateCustomRoleOnProductType(t *testing.T) {
	t.Parallel()
	t.Run(
		"With the project switch on, a project custom role holding project and document permissions is created",
		func(t *testing.T) {
			t.Parallel()
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "project", true)
			sink := w.product.CaptureEvents()

			created := w.create(t, scopedCustomRoleBody(
				"Release manager", "project", "project:manage", "project:archive", "document:view",
			))

			assert.Equal(t, ct.CUSTOM, created.Type)
			assert.Equal(t, "project", created.ResourceType)
			assert.ElementsMatch(t, []string{"project:manage", "project:archive", "document:view"}, grantNames(created))
			assert.Equal(t, "project", w.get(t, created.Id).ResourceType)
			sink.WaitFor("organization.role.created", map[string]string{
				"organization_id": w.organization.Id, "role_id": created.Id,
			})
		},
	)

	t.Run("A project custom role holding an organization permission answers 400", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)

		resp := w.createRaw(t, scopedCustomRoleBody("Release manager", "project", "project:manage", "members:update"))

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "PERMISSION_OUTSIDE_ROLE_SCOPE")
		assert.Equal(t, "organization", meta["permission_resource_type"])
		assert.Equal(t, "project", meta["role_resource_type"])
	})

	t.Run("A folder custom role holding a project permission answers 400", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "folder", true)

		resp := w.createRaw(t, scopedCustomRoleBody("Reviewer", "folder", "folder:edit", "project:manage"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "PERMISSION_OUTSIDE_ROLE_SCOPE")
	})

	t.Run("Changing a project custom role's permissions keeps them within its scope", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)
		created := w.create(t, scopedCustomRoleBody("Release manager", "project", "project:manage"))

		widened := w.updateRaw(t, created.Id, ct.UpdateOrganizationRoleJSONRequestBody{
			Name: created.Name, Permissions: &[]string{"project:manage", "folder:edit"},
		})
		above := w.updateRaw(t, created.Id, ct.UpdateOrganizationRoleJSONRequestBody{
			Name: created.Name, Permissions: &[]string{"members:update"},
		})

		require.Equal(t, http.StatusOK, widened.StatusCode(), string(widened.Body))
		assert.ElementsMatch(t, []string{"project:manage", "folder:edit"}, grantNames(*widened.JSON200))
		requireAPIError(t, above, above.Body, http.StatusBadRequest, "PERMISSION_OUTSIDE_ROLE_SCOPE")
		assert.ElementsMatch(t, []string{"project:manage", "folder:edit"}, grantNames(w.get(t, created.Id)))
	})
}

func TestCustomRoleLimitAcrossTypes(t *testing.T) {
	t.Parallel()
	t.Run("Custom roles of every type count toward one limit of 50, so a 51st on another type answers 409",
		func(t *testing.T) {
			t.Parallel()
			w := newProjectRoleWorld(t)
			for _, slug := range []string{"organization", "project", "folder"} {
				switchCustomRoles(t, w.product, slug, true)
			}
			for i := range 25 {
				w.create(t, scopedCustomRoleBody(fmt.Sprintf("Organization %02d", i), "organization"))
				w.create(t, scopedCustomRoleBody(fmt.Sprintf("Project %02d", i), "project"))
			}

			resp := w.createRaw(t, scopedCustomRoleBody("Folder 00", "folder"))

			requireAPIError(t, resp, resp.Body, http.StatusConflict, "CUSTOM_ROLE_LIMIT_REACHED")
		})
}

func TestProductTypeSwitchOff(t *testing.T) {
	t.Parallel()
	t.Run("Turning the project switch off keeps its custom roles and refuses new ones and changes", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)
		custom := w.create(t, scopedCustomRoleBody("Release manager", "project", "project:manage"))

		switchCustomRoles(t, w.product, "project", false)
		created := w.createRaw(t, scopedCustomRoleBody("Reviewer", "project", "document:view"))
		renamed := w.updateRaw(t, custom.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: "Shipper"})

		requireAPIError(t, created, created.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
		requireAPIError(t, renamed, renamed.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
		kept := w.get(t, custom.Id)
		assert.Equal(t, "Release manager", kept.Name)
		assert.Equal(t, []string{"project:manage"}, grantNames(kept))
	})

	t.Run("With the project switch off, a project custom role can still be deleted", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)
		custom := w.create(t, scopedCustomRoleBody("Release manager", "project"))
		switchCustomRoles(t, w.product, "project", false)

		resp, err := w.client.DeleteOrganizationRoleWithResponse(
			context.Background(), w.product.ProductID, w.organization.Id, custom.Id)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	})
}

// roleLessMember adds a new product user to the world's Organization with no
// role, and returns the user's ID.
func (w projectRoleWorld) roleLessMember(t *testing.T) string {
	t.Helper()
	user := createDSLProductUser(t, w.product)
	resp, err := w.client.AddOrganizationMemberWithResponse(context.Background(), w.product.ProductID,
		w.organization.Id, ct.AddOrganizationMemberJSONRequestBody{ProductUserId: user.ID})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return user.ID
}
