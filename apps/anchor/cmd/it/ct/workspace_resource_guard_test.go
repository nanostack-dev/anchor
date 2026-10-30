package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceResourceRoutesKeepTheWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Deleting a workspace through the resource routes answers 409 and keeps it allowed", func(t *testing.T) {
		t.Parallel()
		w := givenWorkspaceAccessWorld(t)
		resources := w.product.Resources(w.org.Id)

		plain := resources.DeleteRaw(workspaceType, w.private, false)
		cascaded := resources.DeleteRaw(workspaceType, w.shared, true)

		requireAPIError(t, plain, plain.Body, http.StatusConflict, "RESOURCE_IS_WORKSPACE")
		requireAPIError(t, cascaded, cascaded.Body, http.StatusConflict, "RESOURCE_IS_WORKSPACE")
		requireAllowed(t, w.ask(w.editorExternalID, w.shared).Can(w.viewWorkspace).Decide())
		backfill, _ := newWorkspaceBackfill()
		require.NoError(t, backfill.RunProduct(t.Context(), w.product.ProductID))
		assert.Equal(t, "Private", resources.Get(workspaceType, w.private).Name)
	})

	t.Run("The workspace route still deletes the workspace and its resource", func(t *testing.T) {
		t.Parallel()
		w := givenWorkspaceAccessWorld(t)

		w.product.Workspaces(w.org.Id).Delete(w.shared)

		requireNoWorkspaceResource(t, w.product, w.org.Id, w.shared)
		requireDenied(t, w.ask(w.editorExternalID, w.shared).Can(w.viewWorkspace).Decide(),
			ct.WORKSPACENOTINORGANIZATION)
	})
}

func TestMirrorOfADeletedWorkspaceIsAbsent(t *testing.T) {
	t.Parallel()
	w := givenWorkspaceAccessWorld(t)
	resources := w.product.Resources(w.org.Id)
	plain := createManagedRole(t, w.product, ct.CreateProductRoleJSONRequestBody{Name: "Member " + ids.MustNew("ct")})
	editorID, _ := w.memberWithID(t, plain.Id)
	_, err := testDB.ExecContext(t.Context(), `DELETE FROM workspaces WHERE id = $1`, w.shared)
	require.NoError(t, err)

	t.Run("Reading it answers 404", func(t *testing.T) {
		resp := resources.GetRaw(workspaceType, w.shared)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})

	t.Run("A search leaves it out", func(t *testing.T) {
		found := resources.Search(ct.SearchResourcesJSONRequestBody{
			Filter: &ct.ResourceFilter{ResourceTypes: []string{workspaceType}},
		})

		ids := make([]string, 0, len(found.Items))
		for _, item := range found.Items {
			ids = append(ids, item.Id)
		}
		assert.ElementsMatch(t, []string{w.private}, ids)
	})

	t.Run("Assigning, reading or listing roles on it answers 404", func(t *testing.T) {
		members := w.product.ResourceMembers(w.org.Id, workspaceType, w.shared)

		assigned := members.PutRaw(editorID, w.editor.Id)
		read := members.GetRaw(editorID)
		listed := members.SearchRaw(ct.SearchResourceMembersJSONRequestBody{})

		requireAPIError(t, assigned, assigned.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
		requireAPIError(t, read, read.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
		requireAPIError(t, listed, listed.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})
}

func TestWorkspacePrivileges(t *testing.T) {
	t.Parallel()
	w := givenWorkspaceAccessWorld(t)

	t.Run("A role on a workspace allows nothing on its Organization", func(t *testing.T) {
		requireDenied(t, w.product.Access().User(w.editorExternalID).InOrganization(w.org.Id).
			Can(w.viewWorkspace).Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("An Organization API key holding a workspace permission is allowed on its workspace", func(t *testing.T) {
		resp, err := w.product.AllScopeAPIKeyClient().CreateOrganizationAPIKeyWithResponse(
			t.Context(), w.product.ProductID, w.org.Id,
			ct.CreateOrganizationAPIKeyJSONRequestBody{
				Name: "Key-" + ids.MustNew("ct"), Permissions: []string{w.viewWorkspace},
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
		key := w.product.Access().APIKey(resp.JSON201.Value)

		requireAllowed(t, key.InWorkspace("", w.shared).Can(w.viewWorkspace).Decide())
		requireDenied(t, key.InWorkspace(w.other.Id, w.foreign).Can(w.viewWorkspace).Decide(), ct.NOTAMEMBER)
	})

	t.Run("A custom workspace role grants on its workspace only, never on the Organization", func(t *testing.T) {
		switchCustomRoles(t, w.product, workspaceType, true)
		created, err := w.product.AllScopeAPIKeyClient().CreateOrganizationRoleWithResponse(
			t.Context(), w.product.ProductID, w.org.Id, ct.CreateOrganizationRoleJSONRequestBody{
				Name: "Space lead " + ids.MustNew("ct"), ResourceType: workspaceType,
				Permissions: []string{w.viewWorkspace},
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))
		plain := createManagedRole(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member " + ids.MustNew("ct")},
		)
		leadID, leadExternalID := w.memberWithID(t, plain.Id)
		w.product.ResourceMembers(w.org.Id, workspaceType, w.private).Put(leadID, created.JSON201.Id)

		requireAllowed(t, w.ask(leadExternalID, w.private).Can(w.viewWorkspace).Decide())
		requireDenied(t, w.ask(leadExternalID, w.private).Can(w.readReports).Decide(), ct.MISSINGPERMISSION)
		requireDenied(t, w.product.Access().User(leadExternalID).InOrganization(w.org.Id).
			Can(w.viewWorkspace).Decide(), ct.MISSINGPERMISSION)
	})
}

func TestDeprecatedWorkspaceRoutesRefuseAnotherOrganization(t *testing.T) {
	t.Parallel()
	w := givenWorkspaceAccessWorld(t)
	viaOther := w.product.Workspaces(w.other.Id)

	t.Run("Renaming or deleting a workspace through another Organization answers 404 and keeps it", func(t *testing.T) {
		renamed := viaOther.UpdateRaw(w.shared, ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: "Taken"})
		deleted := viaOther.DeleteRaw(w.shared)

		assert.Equal(t, http.StatusNotFound, renamed.StatusCode(), string(renamed.Body))
		assert.Equal(t, http.StatusNotFound, deleted.StatusCode(), string(deleted.Body))
		assert.Equal(t, "Shared", w.product.Resources(w.org.Id).Get(workspaceType, w.shared).Name)
		requireAllowed(t, w.ask(w.editorExternalID, w.shared).Can(w.viewWorkspace).Decide())
	})

	t.Run("Another Product's key can't evaluate a workspace of this Product", func(t *testing.T) {
		other := createTestProductContext(t)

		resp := w.product.AccessClientWith(other.AllScopeAPIKeyClient()).User(w.editorExternalID).
			InWorkspace(w.org.Id, w.shared).Can(w.viewWorkspace).Send()

		requireAPIError(t, resp, resp.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
	})
}
