package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (w memberWorld) assignmentEvent(productUserID, resourceType, resourceID, roleID string) map[string]string {
	return map[string]string{
		"organization_id": w.org.Id, "resource_type": resourceType, "resource_id": resourceID,
		"product_user_id": productUserID, "role_id": roleID,
	}
}

func TestResourceMemberEvents(t *testing.T) {
	t.Parallel()

	t.Run("Assigning, replacing and removing a role reach the event endpoint", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		sink := w.product.CaptureEvents()
		members := w.folderMembers()

		members.Put(w.member, w.editor.Id)
		members.Put(w.member, w.viewer.Id)
		members.Delete(w.member)

		created := sink.WaitFor("resource.role_assignment.created",
			w.assignmentEvent(w.member, "folder", w.folder, w.editor.Id))
		sink.WaitFor("resource.role_assignment.updated", w.assignmentEvent(w.member, "folder", w.folder, w.viewer.Id))
		sink.WaitFor("resource.role_assignment.deleted", w.assignmentEvent(w.member, "folder", w.folder, w.viewer.Id))
		assert.Empty(t, created.Field("resource_row_id"), "Anchor's row ID never leaves Anchor")
	})

	t.Run("Assigning the role a member already holds emits nothing", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		sink := w.product.CaptureEvents()
		w.folderMembers().Put(w.member, w.editor.Id)
		sink.WaitFor("resource.role_assignment.created", w.assignmentEvent(w.member, "folder", w.folder, w.editor.Id))

		w.folderMembers().Put(w.member, w.editor.Id)
		w.folderMembers().Delete(w.member)

		sink.WaitFor("resource.role_assignment.deleted", w.assignmentEvent(w.member, "folder", w.folder, w.editor.Id))
		assert.Equal(t, 1, sink.Count("resource.role_assignment.created"))
		assert.Zero(t, sink.Count("resource.role_assignment.updated"))
	})

	t.Run(
		"The product's event catalog lists the role assignment events and an endpoint can subscribe",
		func(t *testing.T) {
			t.Parallel()
			w := givenMemberWorld(t)
			types := []string{
				"resource.role_assignment.created",
				"resource.role_assignment.updated",
				"resource.role_assignment.deleted",
			}

			resp, err := w.product.OwnerAuthenticatedClient().
				GetProductEventsCatalogWithResponse(t.Context(), w.product.ProductID)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
			var listed []string
			for _, definition := range resp.JSON200.Items {
				listed = append(listed, definition.Type)
			}
			assert.Subset(t, listed, types)

			sink := w.product.CaptureFilteredEvents(types)
			w.folderMembers().Put(w.member, w.editor.Id)
			sink.WaitFor(
				"resource.role_assignment.created",
				w.assignmentEvent(w.member, "folder", w.folder, w.editor.Id),
			)
		},
	)
}

func TestRemoveMemberRemovesResourceRoles(t *testing.T) {
	t.Parallel()

	t.Run(
		"Removing a member from the organization removes their roles on every resource, each with its event",
		func(t *testing.T) {
			t.Parallel()
			w := givenMemberWorld(t)
			stays := w.newMember(t)
			w.folderMembers().Put(w.member, w.editor.Id)
			w.projectMembers().Put(w.member, w.projectAdmin.Id)
			w.folderMembers().Put(stays, w.viewer.Id)
			sink := w.product.CaptureEvents()

			resp, err := w.product.AllScopeAPIKeyClient().RemoveOrganizationMemberWithResponse(
				t.Context(), w.product.ProductID, w.org.Id, w.member)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

			assert.Zero(t, countAssignments(t, w.org.Id, w.member))
			sink.WaitFor(
				"resource.role_assignment.deleted",
				w.assignmentEvent(w.member, "folder", w.folder, w.editor.Id),
			)
			sink.WaitFor("resource.role_assignment.deleted",
				w.assignmentEvent(w.member, "project", w.project, w.projectAdmin.Id))
			sink.WaitFor("organization.membership.deleted", map[string]string{
				"organization_id": w.org.Id, "product_user_id": w.member,
			})
			assert.Equal(t, []string{w.viewer.Id}, roleIDs(w.folderMembers().Get(stays)))
		},
	)
}

func TestDeleteResourceWithRoleAssignments(t *testing.T) {
	t.Parallel()

	t.Run("A resource holding a role assignment answers 409 RESOURCE_IN_USE without cascade", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		w.folderMembers().Put(w.member, w.editor.Id)

		resp := w.resources.DeleteRaw("folder", w.folder, false)

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "RESOURCE_IN_USE")
		assert.InDelta(t, 0, meta["children"], 0)
		assert.InDelta(t, 1, meta["role_assignments"], 0)
	})

	t.Run("A cascade removes the role assignments of the whole subtree, each with its event", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		w.projectMembers().Put(w.member, w.projectAdmin.Id)
		sink := w.product.CaptureEvents()

		w.resources.Delete("project", w.project, true)

		assert.Zero(t, countAssignments(t, w.org.Id, w.member))
		sink.WaitFor("resource.role_assignment.deleted", w.assignmentEvent(w.member, "folder", w.folder, w.editor.Id))
		sink.WaitFor("resource.role_assignment.deleted",
			w.assignmentEvent(w.member, "project", w.project, w.projectAdmin.Id))
	})
}

func TestRoleHeldOnAResource(t *testing.T) {
	t.Parallel()

	t.Run("Deleting a template held on a resource answers 409 ROLE_IN_USE", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		w.folderMembers().Put(w.member, w.editor.Id)

		resp, err := w.product.OwnerAuthenticatedClient().
			DeleteProductRoleWithResponse(t.Context(), w.product.ProductID, w.editor.Id)
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_IN_USE")
	})

	t.Run("Deleting a template no longer held on any resource succeeds", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		w.folderMembers().Delete(w.member)

		resp, err := w.product.OwnerAuthenticatedClient().
			DeleteProductRoleWithResponse(t.Context(), w.product.ProductID, w.editor.Id)
		require.NoError(t, err)

		assert.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	})

	t.Run(
		"With custom roles on, deleting a held template keeps the assignment on the detached copy, "+
			"which can't be deleted while held",
		func(t *testing.T) {
			t.Parallel()
			w := givenMemberWorld(t)
			allowCustomRoles(t, w.product, true)
			w.folderMembers().Put(w.member, w.editor.Id)

			resp, err := w.product.OwnerAuthenticatedClient().
				DeleteProductRoleWithResponse(t.Context(), w.product.ProductID, w.editor.Id)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

			held := w.folderMembers().Get(w.member)
			require.Len(t, held.Roles, 1)
			assert.NotEqual(t, w.editor.Id, held.Roles[0].Id, "the copy is now a custom role under its own ID")
			assert.Equal(t, w.editor.Name, held.Roles[0].Name)
			deleted := w.product.OrganizationRoles(w.org.Id).DeleteRaw(held.Roles[0].Id)
			requireAPIError(t, deleted, deleted.Body, http.StatusConflict, "ROLE_IN_USE")
		},
	)
}

func TestResourceMemberScopes(t *testing.T) {
	t.Parallel()

	t.Run("Assigning and removing need resource_member:write", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		refused := w.folderMembers("resource:read", "resource:write", "resource:delete")
		writer := w.folderMembers("resource_member:write")

		assert.Equal(t, http.StatusForbidden, refused.PutRaw(w.member, w.editor.Id).StatusCode())
		assert.Equal(t, http.StatusOK, writer.PutRaw(w.member, w.editor.Id).StatusCode())
		assert.Equal(t, http.StatusForbidden, refused.DeleteRaw(w.member).StatusCode())
		assert.Equal(t, http.StatusNoContent, writer.DeleteRaw(w.member).StatusCode())
	})

	t.Run("Reading and searching need resource:read", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		writer := w.folderMembers("resource_member:write")
		reader := w.folderMembers("resource:read")

		assert.Equal(t, http.StatusForbidden, writer.GetRaw(w.member).StatusCode())
		assert.Equal(t, http.StatusForbidden, writer.SearchRaw(ct.SearchResourceMembersJSONRequestBody{}).StatusCode())
		assert.Equal(t, http.StatusOK, reader.GetRaw(w.member).StatusCode())
		assert.Equal(t, http.StatusOK, reader.SearchRaw(ct.SearchResourceMembersJSONRequestBody{}).StatusCode())
	})

	t.Run("A platform session reads members, and assigning needs a product API key", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		owner := w.folderMembers().As(w.product.OwnerAuthenticatedClient())

		assert.Equal(t, http.StatusOK, owner.GetRaw(w.member).StatusCode())
		assert.Equal(t, http.StatusOK, owner.SearchRaw(ct.SearchResourceMembersJSONRequestBody{}).StatusCode())
		put, deleted := owner.PutRaw(w.member, w.viewer.Id), owner.DeleteRaw(w.member)
		assert.Equal(t, http.StatusUnauthorized, put.StatusCode(), string(put.Body))
		assert.Equal(t, http.StatusUnauthorized, deleted.StatusCode(), string(deleted.Body))
	})
}
