package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// Every change that takes a privilege away applies to the next decision, and
// nothing gives one back by itself. The member starts with folder-viewer on
// Design, which allows document:view on Brief under it.

func (w resourceAccessWorld) requireBriefAllowed(t *testing.T) {
	t.Helper()
	requireAllowed(t, w.asMember("document", w.document).Can(documentView).Decide())
}

func (w resourceAccessWorld) updateRole(t *testing.T, role ct.ProductRoleResponse, permissions ...string) {
	t.Helper()
	// A nil slice encodes as null, which keeps the role's permissions.
	permissions = append([]string{}, permissions...)
	resp, err := w.product.OwnerAuthenticatedClient().UpdateProductRoleWithResponse(t.Context(), w.product.ProductID,
		role.Id, ct.UpdateProductRoleJSONRequestBody{Name: role.Name, Permissions: &permissions})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
}

func TestPrivilegeChangesReachTheNextDecision(t *testing.T) {
	t.Parallel()

	t.Run("Revoking the folder role denies the folder and the document under it", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		w.requireBriefAllowed(t)

		w.members("folder", w.folder).Delete(w.member)

		requireDenied(t, w.asMember("document", w.document).Can(documentView).Decide(), ct.MISSINGPERMISSION)
		requireDenied(t, w.asMember("folder", w.folder).Can(documentView).Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("Replacing the folder role with one lacking the permission denies it", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		bare := createManagedRole(t, w.product, roleOn("folder", "Folder guest"))

		w.members("folder", w.folder).Put(w.member, bare.Id)

		requireDenied(t, w.asMember("document", w.document).Can(documentView).Decide(), ct.MISSINGPERMISSION)
	})

	t.Run("Removing the permission from the role denies it, and adding it back allows it again", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)

		w.updateRole(t, w.folderViewer)
		denied := w.asMember("document", w.document).Can(documentView).Decide()
		w.updateRole(t, w.folderViewer, documentView)

		requireDenied(t, denied, ct.MISSINGPERMISSION)
		w.requireBriefAllowed(t)
	})

	t.Run("Removing the member from the Organization drops the resource role for good", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)

		resp, err := w.product.AllScopeAPIKeyClient().RemoveOrganizationMemberWithResponse(
			t.Context(), w.product.ProductID, w.org.Id, w.member)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		removed := w.asMember("document", w.document).Can(documentView).Decide()
		addMember(t, w.product, w.org.Id, w.member, w.orgMember.Id)
		rejoined := w.asMember("document", w.document).Can(documentView).Decide()

		requireDenied(t, removed, ct.NOTAMEMBER)
		requireDenied(t, rejoined, ct.MISSINGPERMISSION)
		assert.Equal(t, 0, countAssignments(t, w.org.Id, w.member))
	})

	t.Run("Moving the document out from under the folder denies it, and moving it back allows it", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)

		w.resources.Put("document", w.document, "Brief", itdsl.Ref("project", w.project))
		moved := w.asMember("document", w.document).Can(documentView).Decide()
		w.resources.Put("document", w.document, "Brief", itdsl.Ref("folder", w.folder))

		requireDenied(t, moved, ct.MISSINGPERMISSION)
		w.requireBriefAllowed(t)
	})

	t.Run("Deleting the folder with its subtree denies a document named under it", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)

		w.resources.Delete("folder", w.folder, true)
		named := w.asMember("document", "doc_"+ids.MustNew("ct")).Under("folder", w.folder).Can(documentView).Decide()

		assert.False(t, named.Decision)
	})
}

func TestCustomRoleOnAResource(t *testing.T) {
	t.Parallel()

	t.Run(
		"A custom folder role held on Design allows the document under it, never the project above",
		func(t *testing.T) {
			t.Parallel()
			w := givenResourceAccessWorld(t)
			switchCustomRoles(t, w.product, "folder", true)
			resp, err := w.product.AllScopeAPIKeyClient().CreateOrganizationRoleWithResponse(
				t.Context(), w.product.ProductID, w.org.Id, ct.CreateOrganizationRoleJSONRequestBody{
					Name: "Reviewer " + ids.MustNew("ct"), ResourceType: "folder", Permissions: []string{documentView},
				})
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
			reviewer, reviewerExternal := w.newMember(t, w.orgMember.Id)
			w.members("folder", w.folder).Put(reviewer, resp.JSON201.Id)
			ask := func(resourceType, id string) ct.AccessEvaluationsResponse {
				return w.access.User(reviewerExternal).OnResource(w.org.Id, resourceType, id).Can(documentView).Decide()
			}

			requireAllowed(t, ask("document", w.document))
			requireDenied(t, ask("project", w.project), ct.MISSINGPERMISSION)
		},
	)

	t.Run("A custom role that loses the permission denies it on the next decision", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		switchCustomRoles(t, w.product, "folder", true)
		client := w.product.AllScopeAPIKeyClient()
		created, err := client.CreateOrganizationRoleWithResponse(
			t.Context(), w.product.ProductID, w.org.Id, ct.CreateOrganizationRoleJSONRequestBody{
				Name: "Reviewer " + ids.MustNew("ct"), ResourceType: "folder", Permissions: []string{documentView},
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, created.StatusCode(), string(created.Body))
		w.members("folder", w.folder).Put(w.member, created.JSON201.Id)
		w.requireBriefAllowed(t)

		none := []string{}
		updated, err := client.UpdateOrganizationRoleWithResponse(
			t.Context(),
			w.product.ProductID,
			w.org.Id,
			created.JSON201.Id,
			ct.UpdateOrganizationRoleJSONRequestBody{Name: created.JSON201.Name, Permissions: &none},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, updated.StatusCode(), string(updated.Body))

		requireDenied(t, w.asMember("document", w.document).Can(documentView).Decide(), ct.MISSINGPERMISSION)
	})
}

func TestRoleAssignmentPrivileges(t *testing.T) {
	t.Parallel()

	t.Run("An Organization API key can't assign or remove roles on a resource", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		_, rawKey := w.apiKey(t, documentView)
		asKey := w.product.ClientWithProductKeyHeader(rawKey)

		assigned := w.members("folder", w.folder).As(asKey).PutRaw(w.member, w.documentViewer.Id)
		removed := w.members("folder", w.folder).As(asKey).DeleteRaw(w.member)

		requireAPIError(t, assigned, assigned.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		requireAPIError(t, removed, removed.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		w.requireBriefAllowed(t)
	})

	t.Run("A resource role never changes the member's Organization role", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)

		decision := w.access.User(w.memberExternalID).InOrganization(w.org.Id).Can(documentView).Decide()

		requireDenied(t, decision, ct.MISSINGPERMISSION)
		require.NotNil(t, decision.Context.Role)
		assert.Equal(t, w.orgMember.Id, decision.Context.Role.Id)
	})
}
