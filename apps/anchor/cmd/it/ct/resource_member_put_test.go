package ct_test

import (
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutResourceMember(t *testing.T) {
	t.Parallel()

	t.Run("Assigning a folder role answers the member with that role", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.folderMembers().PutRaw(w.member, w.editor.Id)

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		got := *resp.JSON200
		assert.Equal(t, w.member, got.ProductUserId)
		assert.Equal(t, []ct.ResourceMemberRole{{Id: w.editor.Id, Name: w.editor.Name}}, got.Roles)
		assert.False(t, got.CreatedAt.IsZero())
		assert.Equal(t, got, w.folderMembers().Get(w.member))
	})

	t.Run("A role template's ID names the organization's copy of it", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		w.folderMembers().Put(w.member, w.editor.Id)

		var held string
		require.NoError(
			t,
			testDB.QueryRowContext(t.Context(), `SELECT r.organization_id FROM resource_role_assignments a
			JOIN product_roles r ON r.id = a.role_id WHERE a.organization_id = $1 AND a.product_user_id = $2`,
				w.org.Id, w.member).Scan(&held),
		)
		assert.Equal(t, w.org.Id, held, "the assignment holds the organization's copy, not the template")
	})

	t.Run("A copy's own ID names it too", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		var copyID string
		require.NoError(t, testDB.QueryRowContext(t.Context(),
			`SELECT id FROM product_roles WHERE organization_id = $1 AND template_id = $2`,
			w.org.Id, w.editor.Id).Scan(&copyID))

		got := w.folderMembers().Put(w.member, copyID)

		assert.Equal(t, []string{w.editor.Id}, roleIDs(got), "responses show the template's ID")
	})

	t.Run("A new role replaces the one held and keeps when the member first held one", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		_, err := testDB.ExecContext(t.Context(), `UPDATE resource_role_assignments
			SET created_at = created_at - interval '1 hour', updated_at = updated_at - interval '1 hour'
			WHERE organization_id = $1 AND product_user_id = $2`, w.org.Id, w.member)
		require.NoError(t, err)
		first := w.folderMembers().Get(w.member)

		replaced := w.folderMembers().Put(w.member, w.viewer.Id)

		assert.Equal(t, []string{w.viewer.Id}, roleIDs(replaced))
		assert.True(t, first.CreatedAt.Equal(replaced.CreatedAt))
		assert.WithinDuration(t, time.Now(), replaced.UpdatedAt, time.Minute)
		assert.Equal(t, 1, countAssignments(t, w.org.Id, w.member))
	})

	t.Run("The same role twice leaves the assignment unchanged", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		first := w.folderMembers().Put(w.member, w.editor.Id)

		second := w.folderMembers().Put(w.member, w.editor.Id)

		assert.Equal(t, first, second)
	})

	t.Run("A member holds separate roles on separate resources", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		w.folderMembers().Put(w.member, w.editor.Id)
		w.projectMembers().Put(w.member, w.projectAdmin.Id)

		assert.Equal(t, []string{w.editor.Id}, roleIDs(w.folderMembers().Get(w.member)))
		assert.Equal(t, []string{w.projectAdmin.Id}, roleIDs(w.projectMembers().Get(w.member)))
		assert.Equal(t, 2, countAssignments(t, w.org.Id, w.member))
	})
}

func TestPutResourceMemberRefusals(t *testing.T) {
	t.Parallel()

	t.Run("Two roles answer 400 SINGLE_ROLE_PER_RESOURCE", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.folderMembers().PutRaw(w.member, w.editor.Id, w.viewer.Id)

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "SINGLE_ROLE_PER_RESOURCE")
		assert.InDelta(t, 1, meta["max_roles"], 0)
		assert.Zero(t, countAssignments(t, w.org.Id, w.member))
	})

	t.Run("No role answers 400 VALIDATION_ERROR", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.folderMembers().PutRaw(w.member)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})

	t.Run("Eleven roles answer 400 VALIDATION_ERROR", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		roles := make([]string, 11)
		for i := range roles {
			roles[i] = w.editor.Id
		}

		resp := w.folderMembers().PutRaw(w.member, roles...)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "VALIDATION_ERROR")
	})

	t.Run("A project role on a folder answers 400 ROLE_SCOPE_MISMATCH", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.folderMembers().PutRaw(w.member, w.projectAdmin.Id)

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_SCOPE_MISMATCH")
		assert.Equal(t, "project", meta["role_resource_type"])
		assert.Equal(t, "folder", meta["resource_type"])
	})

	t.Run("An organization role on a folder answers 400 ROLE_SCOPE_MISMATCH", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.folderMembers().PutRaw(w.member, w.orgMember.Id)

		meta := requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_SCOPE_MISMATCH")
		assert.Equal(t, "organization", meta["role_resource_type"])
	})

	t.Run("An unknown role answers 400 ROLE_NOT_FOUND_IN_REQUEST", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.folderMembers().PutRaw(w.member, ids.MustNew("role"))

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_NOT_FOUND_IN_REQUEST")
	})

	t.Run("Another organization's custom role answers 400 ROLE_NOT_FOUND_IN_REQUEST", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		other := w.product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
		var otherCopy string
		require.NoError(t, testDB.QueryRowContext(t.Context(),
			`SELECT id FROM product_roles WHERE organization_id = $1 AND template_id = $2`,
			other.Id, w.editor.Id).Scan(&otherCopy))

		resp := w.folderMembers().PutRaw(w.member, otherCopy)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_NOT_FOUND_IN_REQUEST")
	})

	t.Run("A product user outside the organization answers 404 ORGANIZATION_MEMBERSHIP_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		outsider := createDSLProductUser(t, w.product)

		resp := w.folderMembers().PutRaw(outsider.ID, w.editor.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "ORGANIZATION_MEMBERSHIP_NOT_FOUND")
	})

	t.Run("An unregistered resource answers 404 RESOURCE_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.product.ResourceMembers(w.org.Id, "folder", resourceID()).PutRaw(w.member, w.editor.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})

	t.Run("An undeclared type answers 404 RESOURCE_TYPE_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.product.ResourceMembers(w.org.Id, "spreadsheet", w.folder).PutRaw(w.member, w.editor.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_TYPE_NOT_FOUND")
	})

	t.Run("The organization type answers 404 RESOURCE_TYPE_NOT_REGISTRABLE", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.product.ResourceMembers(w.org.Id, "organization", w.org.Id).PutRaw(w.member, w.orgMember.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_TYPE_NOT_REGISTRABLE")
	})

	t.Run("An organization the product lacks answers 404 ORGANIZATION_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.product.ResourceMembers(ids.MustNew("org"), "folder", w.folder).PutRaw(w.member, w.editor.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "ORGANIZATION_NOT_FOUND")
	})
}

func TestGetResourceMember(t *testing.T) {
	t.Parallel()

	t.Run("A member holding no role on the resource answers 404 RESOURCE_MEMBER_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		w.projectMembers().Put(w.member, w.projectAdmin.Id)

		resp := w.folderMembers().GetRaw(w.member)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_MEMBER_NOT_FOUND")
	})

	t.Run("A role held on the parent is not listed on the child", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		w.projectMembers().Put(w.member, w.projectAdmin.Id)

		found := w.folderMembers().Search(ct.SearchResourceMembersJSONRequestBody{})

		assert.Empty(t, found.Items)
	})
}

func TestDeleteResourceMember(t *testing.T) {
	t.Parallel()

	t.Run("Removing a member's role keeps them in the organization and on other resources", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		w.projectMembers().Put(w.member, w.projectAdmin.Id)

		w.folderMembers().Delete(w.member)

		resp := w.folderMembers().GetRaw(w.member)
		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_MEMBER_NOT_FOUND")
		assert.Equal(t, []string{w.projectAdmin.Id}, roleIDs(w.projectMembers().Get(w.member)))
		got, err := w.product.AllScopeAPIKeyClient().GetOrganizationMemberWithResponse(
			t.Context(), w.product.ProductID, w.org.Id, w.member, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, got.StatusCode(), string(got.Body))
	})

	t.Run("Removing a member holding no role answers 404 RESOURCE_MEMBER_NOT_FOUND", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)

		resp := w.folderMembers().DeleteRaw(w.member)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_MEMBER_NOT_FOUND")
	})
}
