package ct_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// releaseManagerWorld is a project custom role holding a project and a
// document permission, so moving document out from under project leaves the
// role holding a permission outside its scope.
type releaseManagerWorld struct {
	projectRoleWorld
	role ct.ProductRoleResponse
}

func newReleaseManagerWorld(t *testing.T) releaseManagerWorld {
	t.Helper()
	w := newProjectRoleWorld(t)
	switchCustomRoles(t, w.product, "project", true)
	role := w.create(t, scopedCustomRoleBody("Release manager", "project", "project:manage", "document:view"))
	return releaseManagerWorld{projectRoleWorld: w, role: role}
}

func (w releaseManagerWorld) moveDocumentToOrganization(t *testing.T) *ct.UpdateProductResourceTypeResponse {
	t.Helper()
	types := w.product.ResourceTypes()
	return types.UpdateRaw(types.BySlug("document").Id, ct.UpdateProductResourceTypeJSONRequestBody{
		ParentTypes: &[]string{"organization"},
	})
}

func (w projectRoleWorld) deleteDocumentView(t *testing.T) {
	t.Helper()
	resp, err := w.client.DeleteProductResourcePermissionWithResponse(
		context.Background(), w.product.ProductID, "document:view")
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
}

func TestProductTreeChangeWithCustomRoles(t *testing.T) {
	t.Parallel()

	t.Run("Moving a type out of a custom role's scope removes its permissions from the role", func(t *testing.T) {
		t.Parallel()
		w := newReleaseManagerWorld(t)
		sink := w.product.CaptureEvents()

		resp := w.moveDocumentToOrganization(t)

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.Equal(t, []string{"project:manage"}, grantNames(w.get(t, w.role.Id)))
		sink.WaitFor("organization.role.updated", map[string]string{
			"organization_id": w.organization.Id, "role_id": w.role.Id,
		})
	})

	t.Run("Publishing a tree that moves the type removes the permissions the same way", func(t *testing.T) {
		t.Parallel()
		w := newReleaseManagerWorld(t)

		resp := w.product.ResourceTypes().ReplaceTreeRaw(
			ct.ProductResourceTypeTreeItem{Slug: "project", Name: "Project", ParentTypes: []string{"organization"}},
			ct.ProductResourceTypeTreeItem{
				Slug: "folder", Name: "Folder", ParentTypes: []string{"project"}, NestsInItself: new(true),
			},
			ct.ProductResourceTypeTreeItem{Slug: "document", Name: "Document", ParentTypes: []string{"organization"}},
		)

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.Equal(t, []string{"project:manage"}, grantNames(w.get(t, w.role.Id)))
	})

	t.Run("A managed role still blocks the change, and the 409 names it alone", func(t *testing.T) {
		t.Parallel()
		w := newReleaseManagerWorld(t)
		managed := createScopedRole(t, w.product, "project", "document:view")

		resp := w.moveDocumentToOrganization(t)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
		assert.Equal(t, []any{managed.Id}, meta["role_ids"])
		assert.ElementsMatch(t, []string{"project:manage", "document:view"}, grantNames(w.get(t, w.role.Id)))
	})
}

func TestDeleteResourceTypeWithCustomRoles(t *testing.T) {
	t.Parallel()

	t.Run("Deleting a type deletes its custom roles nobody holds", func(t *testing.T) {
		t.Parallel()
		w := newProjectRoleWorld(t)
		switchCustomRoles(t, w.product, "document", true)
		reader := w.create(t, scopedCustomRoleBody("Reader", "document", "document:view"))
		w.deleteDocumentView(t)
		sink := w.product.CaptureEvents()
		types := w.product.ResourceTypes()

		resp := types.DeleteRaw(types.BySlug("document").Id)

		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		found, err := w.client.GetOrganizationRoleWithResponse(
			context.Background(), w.product.ProductID, w.organization.Id, reader.Id)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, found.StatusCode())
		sink.WaitFor("organization.role.deleted", map[string]string{
			"organization_id": w.organization.Id, "role_id": reader.Id,
		})
	})

	t.Run(
		"A custom role a member holds on a resource blocks the delete, counted in custom_roles_held",
		func(t *testing.T) {
			t.Parallel()
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "document", true)
			reader := w.create(t, scopedCustomRoleBody("Reader", "document", "document:view"))
			member := w.roleLessMember(t)
			resources := w.product.Resources(w.organization.Id)
			resources.Put("project", "launch", "Launch", resources.Top())
			resources.Put("document", "brief", "Brief", itdsl.Ref("project", "launch"))
			w.product.ResourceMembers(w.organization.Id, "document", "brief").Put(member, reader.Id)
			w.deleteDocumentView(t)
			types := w.product.ResourceTypes()

			resp := types.DeleteRaw(types.BySlug("document").Id)

			meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
			assert.Equal(t, map[string]any{
				"child_types": float64(0), "roles": float64(0), "permissions": float64(0),
				"resources": float64(1), "custom_roles_held": float64(1),
			}, meta["blocked_by"])
			assert.Equal(t, "document", w.get(t, reader.Id).ResourceType)
		},
	)

	// Since ADR-0022 D4, a membership or an invitation holds an
	// organization-scoped role only, so these rows can only be older: SQL
	// writes them.
	t.Run(
		"A membership written before Organization roles had to be organization-scoped still blocks the delete",
		func(t *testing.T) {
			t.Parallel()
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "document", true)
			reader := w.create(t, scopedCustomRoleBody("Reader", "document", "document:view"))
			member := w.roleLessMember(t)
			_, err := testDB.ExecContext(
				t.Context(),
				`UPDATE organization_memberships SET product_role_id = $1 WHERE organization_id = $2 AND product_user_id = $3`,
				reader.Id,
				w.organization.Id,
				member,
			)
			require.NoError(t, err)
			w.deleteDocumentView(t)
			types := w.product.ResourceTypes()

			resp := types.DeleteRaw(types.BySlug("document").Id)

			meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
			assert.Equal(t, map[string]any{
				"child_types": float64(
					0,
				),
				"roles":             float64(0),
				"permissions":       float64(0),
				"custom_roles_held": float64(1),
			}, meta["blocked_by"])
		},
	)

	t.Run(
		"A pending invitation written before Organization roles had to be organization-scoped still blocks the delete",
		func(t *testing.T) {
			t.Parallel()
			w := newProjectRoleWorld(t)
			switchCustomRoles(t, w.product, "document", true)
			reader := w.create(t, scopedCustomRoleBody("Reader", "document", "document:view"))
			invitation := w.product.Invitations().
				Create(w.organization.Id, ct.CreateOrganizationInvitationJSONRequestBody{
					Email: openapi_types.Email("ana-" + uuid.NewString() + "@example.invalid"),
				})
			_, err := testDB.ExecContext(t.Context(),
				`UPDATE organization_invitations SET product_role_id = $1 WHERE id = $2`, reader.Id, invitation.Id)
			require.NoError(t, err)
			w.deleteDocumentView(t)
			types := w.product.ResourceTypes()

			resp := types.DeleteRaw(types.BySlug("document").Id)

			meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_TYPE_IN_USE")
			assert.EqualValues(t, 1, meta["blocked_by"].(map[string]any)["custom_roles_held"])
		},
	)
}
