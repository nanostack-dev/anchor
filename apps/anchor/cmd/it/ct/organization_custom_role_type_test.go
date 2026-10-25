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

// switchCustomRoles sets the custom_roles switch of one resource type.
func switchCustomRoles(
	t *testing.T, product *itdsl.ProductContext, slug string, on bool,
) ct.ProductResourceTypeResponse {
	t.Helper()
	types := product.ResourceTypes()
	return types.Update(types.BySlug(slug).Id, ct.UpdateProductResourceTypeJSONRequestBody{CustomRoles: &on})
}

func scopedCustomRoleBody(name, resourceType string, permissions ...string) ct.CreateOrganizationRoleJSONRequestBody {
	return ct.CreateOrganizationRoleJSONRequestBody{Name: name, ResourceType: resourceType, Permissions: permissions}
}

// projectRoleWorld is a product with the document tree, one permission scoped
// to each of its types, a "Project lead" template on project bundling
// project:manage, and one organization.
type projectRoleWorld struct {
	product      *itdsl.ProductContext
	lead         ct.ProductRoleResponse
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
		product: product,
		lead: createRoleTemplate(t, product, ct.CreateProductRoleJSONRequestBody{
			Name: "Project lead", ResourceType: "project", Permissions: []string{"project:manage"},
		}),
		organization: product.CreateOrganization(t, "Org-"+uuid.NewString(), nil),
	}
}

func (w projectRoleWorld) roles() itdsl.OrganizationRoleClient {
	return w.product.OrganizationRoles(w.organization.Id)
}

func TestCustomRolesSwitchPerType(t *testing.T) {
	t.Parallel()
	t.Run("Each type keeps its own switch: project on leaves folder and organization off", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)

		switchCustomRoles(t, w.product, "project", true)

		types := w.product.ResourceTypes()
		assert.True(t, types.BySlug("project").CustomRoles)
		assert.False(t, types.BySlug("folder").CustomRoles)
		assert.False(t, types.BySlug("organization").CustomRoles)
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
		assert.True(t, published[1].CustomRoles)
	})

	t.Run("With the organization switch on and project off, a project custom role answers 400", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "organization", true)

		resp := w.roles().CreateRaw(scopedCustomRoleBody("Release manager", "project", "project:manage"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
	})

	t.Run(
		"With the project switch on and organization off, an organization custom role answers 400",
		func(t *testing.T) {
			t.Parallel()
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "project", true)

			resp := w.roles().CreateRaw(scopedCustomRoleBody("Billing auditor", "organization", "members:update"))

			requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
		},
	)

	t.Run("With the folder switch on, a project custom role still answers 400", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "folder", true)

		resp := w.roles().CreateRaw(scopedCustomRoleBody("Release manager", "project", "project:manage"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
	})
}

func TestCreateCustomRoleOnProductType(t *testing.T) {
	t.Parallel()
	t.Run(
		"With the project switch on, a project custom role bundling project and document permissions is created",
		func(t *testing.T) {
			t.Parallel()
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "project", true)
			sink := w.product.CaptureEvents()

			created := w.roles().Create(scopedCustomRoleBody(
				"Release manager", "project", "project:manage", "project:archive", "document:view",
			))

			assert.Equal(t, ct.OrganizationRoleOrigin("custom"), created.Origin)
			assert.Equal(t, "project", created.ResourceType)
			assert.ElementsMatch(t, []string{"project:manage", "project:archive", "document:view"}, created.Permissions)
			assert.Equal(t, "project", w.roles().Get(created.Id).ResourceType)
			sink.WaitFor("organization.role.created", map[string]string{
				"organization_id": w.organization.Id, "role_id": created.Id,
			})
		},
	)

	t.Run("A project custom role bundling an organization permission answers 400", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)

		resp := w.roles().
			CreateRaw(scopedCustomRoleBody("Release manager", "project", "project:manage", "members:update"))

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "PERMISSION_OUTSIDE_ROLE_SCOPE")
		assert.Equal(t, "organization", meta["permission_resource_type"])
		assert.Equal(t, "project", meta["role_resource_type"])
	})

	t.Run("A folder custom role bundling a project permission answers 400", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "folder", true)

		resp := w.roles().CreateRaw(scopedCustomRoleBody("Reviewer", "folder", "folder:edit", "project:manage"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "PERMISSION_OUTSIDE_ROLE_SCOPE")
	})

	t.Run("A project custom role can't become the organization's default", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)
		created := w.roles().Create(scopedCustomRoleBody("Release manager", "project", "project:manage"))

		resp := w.roles().UpdateRaw(created.Id, ct.UpdateOrganizationRoleJSONRequestBody{IsDefault: new(true)})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "DEFAULT_ROLE_NOT_ORGANIZATION_SCOPED")
	})

	t.Run("Changing a project custom role's permissions keeps them within its scope", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)
		created := w.roles().Create(scopedCustomRoleBody("Release manager", "project", "project:manage"))

		widened := w.roles().Update(created.Id, ct.UpdateOrganizationRoleJSONRequestBody{
			Permissions: &[]string{"project:manage", "folder:edit"},
		})
		above := w.roles().UpdateRaw(created.Id, ct.UpdateOrganizationRoleJSONRequestBody{
			Permissions: &[]string{"members:update"},
		})

		assert.ElementsMatch(t, []string{"project:manage", "folder:edit"}, widened.Permissions)
		requireAPIError(t, above, above.Body, http.StatusBadRequest, "PERMISSION_OUTSIDE_ROLE_SCOPE")
		assert.ElementsMatch(t, []string{"project:manage", "folder:edit"}, w.roles().Get(created.Id).Permissions)
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
				w.roles().Create(scopedCustomRoleBody(fmt.Sprintf("Organization %02d", i), "organization"))
				w.roles().Create(scopedCustomRoleBody(fmt.Sprintf("Project %02d", i), "project"))
			}

			resp := w.roles().CreateRaw(scopedCustomRoleBody("Folder 00", "folder"))

			requireAPIError(t, resp, resp.Body, http.StatusConflict, "CUSTOM_ROLE_LIMIT_REACHED")
		})
}

func TestProductTypeSwitchOff(t *testing.T) {
	t.Parallel()
	t.Run("Turning the project switch off keeps its custom roles and refuses new ones and changes", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)
		custom := w.roles().Create(scopedCustomRoleBody("Release manager", "project", "project:manage"))
		w.roles().Update(w.lead.Id, ct.UpdateOrganizationRoleJSONRequestBody{
			Permissions: &[]string{"project:manage", "document:view"},
		})

		switchCustomRoles(t, w.product, "project", false)
		created := w.roles().CreateRaw(scopedCustomRoleBody("Reviewer", "project", "document:view"))
		renamed := w.roles().UpdateRaw(custom.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: new("Shipper")})

		requireAPIError(t, created, created.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
		requireAPIError(t, renamed, renamed.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
		kept := w.roles().Get(custom.Id)
		assert.Equal(t, "Release manager", kept.Name)
		assert.Equal(t, []string{"project:manage"}, kept.Permissions)
		copied := w.roles().Get(w.lead.Id)
		assert.True(t, copied.Customized)
		assert.ElementsMatch(t, []string{"project:manage", "document:view"}, copied.Permissions)
	})

	t.Run("With the project switch off, a project custom role can still be deleted", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)
		custom := w.roles().Create(scopedCustomRoleBody("Release manager", "project"))
		switchCustomRoles(t, w.product, "project", false)

		resp := w.roles().DeleteRaw(custom.Id)

		assert.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	})
}

func TestCustomizeProductTypeCopy(t *testing.T) {
	t.Parallel()
	t.Run("With the organization switch on and project off, changing a project copy answers 400", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "organization", true)

		resp := w.roles().UpdateRaw(w.lead.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: new("Lead")})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
	})

	t.Run("With the project switch on, a customized project copy keeps its bundle when the template changes",
		func(t *testing.T) {
			t.Parallel()
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "project", true)
			other := w.product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)

			customized := w.roles().Update(w.lead.Id, ct.UpdateOrganizationRoleJSONRequestBody{
				Name: new("Lead"), Permissions: &[]string{"project:manage", "document:view"},
			})
			resp, err := w.product.OwnerAuthenticatedClient().AssignPermissionToProductRoleWithResponse(t.Context(),
				w.product.ProductID, w.lead.Id,
				ct.AssignPermissionToProductRoleJSONRequestBody{PermissionName: "project:archive"})
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

			assert.True(t, customized.Customized)
			assert.Equal(t, "project", customized.ResourceType)
			assert.ElementsMatch(t, []string{"project:manage", "document:view"}, w.roles().Get(w.lead.Id).Permissions)
			assert.ElementsMatch(t, []string{"project:manage", "project:archive"},
				w.product.OrganizationRoles(other.Id).Get(w.lead.Id).Permissions)
		})

	t.Run("A reset project copy follows its template again, with the project switch off", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)
		w.roles().Update(w.lead.Id, ct.UpdateOrganizationRoleJSONRequestBody{
			Name: new("Lead"), Permissions: &[]string{"document:view"},
		})
		switchCustomRoles(t, w.product, "project", false)
		sink := w.product.CaptureEvents()

		reset := w.roles().Reset(w.lead.Id)

		assert.False(t, reset.Customized)
		assert.Equal(t, "Project lead", reset.Name)
		assert.Equal(t, []string{"project:manage"}, reset.Permissions)
		sink.WaitFor("organization.role.updated", map[string]string{
			"organization_id": w.organization.Id, "role_id": w.lead.Id,
		})
	})
}

func TestDeleteProductTypeTemplate(t *testing.T) {
	t.Parallel()
	t.Run(
		"With only the project switch on, a customized project copy becomes a custom role when its template is deleted",
		func(t *testing.T) {
			t.Parallel()
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "project", true)
			w.roles().Update(w.lead.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: new("Lead")})

			resp, err := w.product.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(t.Context(),
				w.product.ProductID, w.lead.Id)
			require.NoError(t, err)

			require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
			listed := w.roles().Search(ct.SearchOrganizationRolesJSONRequestBody{})
			require.Len(t, listed.Items, 1)
			assert.Equal(t, "Lead", listed.Items[0].Name)
			assert.Equal(t, "project", listed.Items[0].ResourceType)
			assert.Equal(t, ct.OrganizationRoleOrigin("custom"), listed.Items[0].Origin)
		},
	)

	t.Run("With every switch off, deleting a project template removes its customized copy", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "project", true)
		w.roles().Update(w.lead.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: new("Lead")})
		switchCustomRoles(t, w.product, "project", false)

		resp, err := w.product.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(t.Context(),
			w.product.ProductID, w.lead.Id)
		require.NoError(t, err)

		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		assert.Empty(t, w.roles().Search(ct.SearchOrganizationRolesJSONRequestBody{}).Items)
	})
}

// Not parallel: it asserts lock waits within tight timeouts. A row the test
// holds pauses one write inside its transaction while the other one starts.
func TestCustomRoleCreateDuringATypeWrite(t *testing.T) {
	t.Run("A project custom role created while the switch turns off commits first, and the next one answers 400",
		func(t *testing.T) {
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "project", true)
			project := w.product.ResourceTypes().BySlug("project")
			releaseOrganization := holdRowForUpdate(t, "organizations", w.organization.Id)

			created := createCustomRoleInBackground(w, scopedCustomRoleBody("Release manager", "project"))
			waitUntilStatementWaitsOnALock(t, "FOR NO KEY UPDATE", "public.organizations")
			switched := updateResourceTypeInBackground(w.product, project.Id, false)
			waitUntilStatementWaitsOnALock(t, "FOR NO KEY UPDATE", "public.product_resource_types")
			releaseOrganization()

			createdResp := <-created
			assert.Equal(t, http.StatusCreated, createdResp.StatusCode(), string(createdResp.Body))
			switchedResp := <-switched
			assert.Equal(t, http.StatusOK, switchedResp.StatusCode(), string(switchedResp.Body))
			require.NotNil(t, createdResp.JSON201)
			assert.Equal(t, "Release manager", w.roles().Get(createdResp.JSON201.Id).Name)
			next := w.roles().CreateRaw(scopedCustomRoleBody("Reviewer", "project"))
			requireAPIError(t, next, next.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
		})

	t.Run("A project custom role created while the switch is turning off waits for it and answers 400",
		func(t *testing.T) {
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "project", true)
			project := w.product.ResourceTypes().BySlug("project")
			releaseProject := holdRowForUpdate(t, "product_resource_types", project.Id)

			switched := updateResourceTypeInBackground(w.product, project.Id, false)
			waitUntilStatementWaitsOnALock(t, "UPDATE public.product_resource_types", "custom_roles")
			created := createCustomRoleInBackground(w, scopedCustomRoleBody("Release manager", "project"))
			waitUntilStatementWaitsOnALock(t, "FOR SHARE", "public.product_resource_types")
			releaseProject()

			switchedResp := <-switched
			assert.Equal(t, http.StatusOK, switchedResp.StatusCode(), string(switchedResp.Body))
			createdResp := <-created
			requireAPIError(t, createdResp, createdResp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
			assert.Empty(t, w.roles().Search(ct.SearchOrganizationRolesJSONRequestBody{
				Filter: &ct.OrganizationRoleFilter{
					ResourceTypes: []string{"project"},
					Keys:          []string{"release manager"},
				},
			}).Items)
		})

	t.Run("A project type deleted while a project custom role is created answers 409 and keeps the role",
		func(t *testing.T) {
			w := newProjectRoleWorld(t)
			types := w.product.ResourceTypes()
			lone := types.CreateUnder("lone", "organization")
			switchCustomRoles(t, w.product, lone.Slug, true)
			releaseOrganization := holdRowForUpdate(t, "organizations", w.organization.Id)

			created := createCustomRoleInBackground(w, scopedCustomRoleBody("Lone keeper", lone.Slug))
			waitUntilStatementWaitsOnALock(t, "FOR NO KEY UPDATE", "public.organizations")
			deleted := deleteResourceTypeInBackground(w.product, lone.Id)
			waitUntilStatementWaitsOnALock(t, "FOR NO KEY UPDATE", "public.product_resource_types")
			releaseOrganization()

			createdResp := <-created
			assert.Equal(t, http.StatusCreated, createdResp.StatusCode(), string(createdResp.Body))
			deletedResp := <-deleted
			meta := requireAPIError(t, deletedResp, deletedResp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
			assert.Equal(t, map[string]any{"child_types": float64(0), "roles": float64(1), "permissions": float64(0)},
				meta["blocked_by"])
		})
}

func createCustomRoleInBackground(
	w projectRoleWorld, body ct.CreateOrganizationRoleJSONRequestBody,
) <-chan *ct.CreateOrganizationRoleResponse {
	client := w.product.AllScopeAPIKeyClient()
	created := make(chan *ct.CreateOrganizationRoleResponse, 1)
	go func() {
		resp, err := client.CreateOrganizationRoleWithResponse(context.Background(),
			w.product.ProductID, w.organization.Id, body)
		if err != nil {
			resp = &ct.CreateOrganizationRoleResponse{Body: []byte(err.Error())}
		}
		created <- resp
	}()
	return created
}

func updateResourceTypeInBackground(
	product *itdsl.ProductContext, id string, customRoles bool,
) <-chan *ct.UpdateProductResourceTypeResponse {
	client := product.AllScopeAPIKeyClient()
	updated := make(chan *ct.UpdateProductResourceTypeResponse, 1)
	go func() {
		resp, err := client.UpdateProductResourceTypeWithResponse(context.Background(), product.ProductID, id,
			ct.UpdateProductResourceTypeJSONRequestBody{CustomRoles: &customRoles})
		if err != nil {
			resp = &ct.UpdateProductResourceTypeResponse{Body: []byte(err.Error())}
		}
		updated <- resp
	}()
	return updated
}

func deleteResourceTypeInBackground(
	product *itdsl.ProductContext, id string,
) <-chan *ct.DeleteProductResourceTypeResponse {
	client := product.AllScopeAPIKeyClient()
	deleted := make(chan *ct.DeleteProductResourceTypeResponse, 1)
	go func() {
		resp, err := client.DeleteProductResourceTypeWithResponse(context.Background(), product.ProductID, id)
		if err != nil {
			resp = &ct.DeleteProductResourceTypeResponse{Body: []byte(err.Error())}
		}
		deleted <- resp
	}()
	return deleted
}
