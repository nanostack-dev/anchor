package ct_test

import (
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnchorSDKResources drives a product's resource tree, the roles held on
// it and access to it through anchorsdk, against the running server.
func TestAnchorSDKResources(t *testing.T) {
	t.Parallel()

	t.Run("A resource put registers, renames and moves it", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		org := w.product.SDKClient().Organization(w.org.Id)
		folder := org.Resource("folder", resourceID())

		created, err := folder.Put("Drafts").Under("project", w.project).Do(t.Context())
		require.NoError(t, err)
		_, err = folder.Put("Drafts").Under("project", w.project).Do(t.Context())
		require.NoError(t, err, "the same put again changes nothing")
		moved, err := folder.Put("Archive").Under("folder", w.folder).Do(t.Context())
		require.NoError(t, err)
		read, err := folder.Get(t.Context())
		require.NoError(t, err)

		assert.Equal(t, "project", created.Parent.Type)
		assert.Equal(t, w.folder, moved.Parent.Id)
		assert.Equal(t, "Archive", read.Name)
		assert.Equal(t, created.CreatedAt, read.CreatedAt)
	})

	t.Run("A put without a parent sits under the organization", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		org := w.product.SDKClient().Organization(w.org.Id)

		created, err := org.Resource("project", resourceID()).Put("Launch").Do(t.Context())

		require.NoError(t, err)
		assert.Equal(t, "organization", created.Parent.Type)
		assert.Equal(t, w.org.Id, created.Parent.Id)
	})

	t.Run("A search reads every page of one parent's children", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		org := w.product.SDKClient().Organization(w.org.Id)

		found := []string{}
		for item, err := range org.Resources().Search().Under("folder", w.folder).Limit(1).All(t.Context()) {
			require.NoError(t, err)
			found = append(found, item.Type+"/"+item.Id)
		}

		assert.ElementsMatch(t, []string{"folder/" + w.subfolder, "document/" + w.document}, found)
	})

	t.Run("A role assigned through the SDK decides access below the resource, and removing it takes it back",
		func(t *testing.T) {
			t.Parallel()
			w := givenResourceAccessWorld(t)
			sdk := w.product.SDKClient()
			org := sdk.Organization(w.org.Id)
			reader, readerExternalID := w.newMember(t, w.orgMember.Id)
			subfolder := org.Resource("folder", w.subfolder)
			asReader := func(r *anchorsdk.Resource) bool {
				ok, err := sdk.AccessForUser(readerExternalID).On(r).Can(t.Context(), documentView)
				require.NoError(t, err)
				return ok
			}

			assigned, err := subfolder.Members().Assign(t.Context(), reader, w.folderViewer.Id)
			require.NoError(t, err)

			assert.Equal(t, w.folderViewer.Id, assigned.Roles[0].Id)
			assert.True(t, asReader(org.Resource("document", w.subDocument)), "the role reaches a document below")
			assert.False(t, asReader(org.Resource("document", w.document)), "the role stops at its resource")

			require.NoError(t, subfolder.Members().Remove(t.Context(), reader))

			assert.False(t, asReader(org.Resource("document", w.subDocument)))
			_, err = subfolder.Members().Get(t.Context(), reader)
			assert.ErrorIs(t, err, anchorsdk.ErrNotFound)
		})

	t.Run("A member search lists who holds roles on a resource", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		folder := w.product.SDKClient().Organization(w.org.Id).Resource("folder", w.folder)

		page, err := folder.Members().Search().RoleIDs(w.folderViewer.Id).Do(t.Context())

		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		assert.Equal(t, w.member, page.Items[0].ProductUserId)
	})

	t.Run("An API key's access is decided on a resource handle", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		sdk := w.product.SDKClient()
		_, key := w.apiKey(t, documentView)
		document := sdk.Organization(w.org.Id).Resource("document", w.document)

		canView, err := sdk.AccessForAPIKey(key).On(document).Can(t.Context(), documentView)
		require.NoError(t, err)
		canEdit, err := sdk.AccessForAPIKey(key).On(document).Can(t.Context(), documentEdit)
		require.NoError(t, err)

		assert.True(t, canView)
		assert.False(t, canEdit)
	})

	t.Run("A delete refuses a resource that only holds a role", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		org := w.product.SDKClient().Organization(w.org.Id)
		document := org.Resource("document", w.document)
		_, err := document.Members().Assign(t.Context(), w.member, w.documentViewer.Id)
		require.NoError(t, err)

		require.ErrorIs(t, document.Delete(t.Context()), anchorsdk.ErrConflict)
		require.NoError(t, document.DeleteTree(t.Context()))

		_, err = document.Get(t.Context())
		assert.ErrorIs(t, err, anchorsdk.ErrNotFound)
	})

	t.Run("A put under a parent that is not registered is a conflict", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		org := w.product.SDKClient().Organization(w.org.Id)

		_, err := org.Resource("folder", resourceID()).Put("Orphan").Under("project", resourceID()).Do(t.Context())

		require.ErrorIs(t, err, anchorsdk.ErrConflict)
	})

	t.Run("A delete refuses a resource that holds others, and a tree delete removes them", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		org := w.product.SDKClient().Organization(w.org.Id)
		folder := org.Resource("folder", w.folder)

		refused := folder.Delete(t.Context())
		require.ErrorIs(t, refused, anchorsdk.ErrConflict)
		require.NoError(t, folder.DeleteTree(t.Context()))

		for _, gone := range []*anchorsdk.Resource{folder, org.Resource("document", w.subDocument)} {
			_, err := gone.Get(t.Context())
			require.ErrorIs(t, err, anchorsdk.ErrNotFound)
		}
		_, err := org.Resource("project", w.project).Get(t.Context())
		assert.NoError(t, err, "the parent stays")
	})
}

// TestAnchorSDKWorkspaceResources is the product that keeps its workspaces as
// resources of its own workspace type, registered through the SDK.
func TestAnchorSDKWorkspaceResources(t *testing.T) {
	t.Parallel()

	t.Run("A workspace registered through the SDK decides access by the roles held on it", func(t *testing.T) {
		t.Parallel()
		w := givenWorkspaceAccessWorld(t)
		sdk := w.product.SDKClient()
		org := sdk.Organization(w.org.Id)
		ws := org.Resource(workspaceType, "ws_"+resourceID())
		plain := createManagedRole(
			t,
			w.product,
			ct.CreateProductRoleJSONRequestBody{Name: "Member " + ids.MustNew("ct")},
		)
		leadID, leadExternalID := w.memberWithID(t, plain.Id)
		can := func(externalID string, r *anchorsdk.Resource) bool {
			ok, err := sdk.AccessForUser(externalID).On(r).Can(t.Context(), w.viewWorkspace)
			require.NoError(t, err)
			return ok
		}

		_, err := ws.Put("Default").Do(t.Context())
		require.NoError(t, err)
		assert.False(t, can(leadExternalID, ws), "a member with no role on it")
		_, err = ws.Members().Assign(t.Context(), leadID, w.editor.Id)
		require.NoError(t, err)

		assert.True(t, can(leadExternalID, ws))
		assert.True(t, can(w.adminExternalID, ws), "the Organization role reaches it")
		assert.False(t, can(leadExternalID, org.Resource(workspaceType, w.private)), "the role stops at its workspace")
		found := []string{}
		for item, err := range org.Resources().Search().Types(workspaceType).All(t.Context()) {
			require.NoError(t, err)
			found = append(found, item.Id)
		}
		assert.ElementsMatch(t, []string{ws.ID(), w.shared, w.private}, found)
	})

	t.Run("A workspace of the deprecated routes can't be deleted as a resource", func(t *testing.T) {
		t.Parallel()
		w := givenWorkspaceAccessWorld(t)
		shared := w.product.SDKClient().Organization(w.org.Id).Resource(workspaceType, w.shared)

		require.ErrorIs(t, shared.Delete(t.Context()), anchorsdk.ErrConflict)
		require.ErrorIs(t, shared.DeleteTree(t.Context()), anchorsdk.ErrConflict)
	})
}
