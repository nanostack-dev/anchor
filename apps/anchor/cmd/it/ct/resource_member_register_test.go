package ct_test

import (
	"sync"
	"testing"

	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The member PUT with a resource block is driven through anchorsdk's
// Put(...).Under(...).Assign, so each case tests the route and the SDK call
// that wraps it.

// requireSDKError asserts a refused SDK call's sentinel and its one error
// code, and returns the error's metadata.
func requireSDKError(t *testing.T, err error, sentinel error, code string) map[string]any {
	t.Helper()
	require.ErrorIs(t, err, sentinel)
	var apiErr *anchorsdk.Error
	require.ErrorAs(t, err, &apiErr)
	require.Len(t, apiErr.Details, 1, string(apiErr.Body))
	require.Equal(t, code, apiErr.Details[0].Code, string(apiErr.Body))
	return apiErr.Details[0].Metadata
}

// documentHandle is a document of the world's first Organization, through the
// SDK client.
func (w resourceAccessWorld) documentHandle(sdk *anchorsdk.Client, id string) *anchorsdk.Resource {
	return sdk.Organization(w.org.Id).Resource("document", id)
}

func TestResourceMemberPutRegistersTheResource(t *testing.T) {
	t.Parallel()

	t.Run("One SDK call registers a document under its folder and gives the member a role on it", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, danExternalID := w.newMember(t, w.orgMember.Id)
		sink := w.product.CaptureEvents()
		pricing := w.documentHandle(w.product.SDKClient(), resourceID())

		assigned, err := pricing.Put("Pricing page").Under("folder", w.folder).
			Assign(t.Context(), dan, w.documentViewer.Id)

		require.NoError(t, err)
		assert.Equal(t, w.documentViewer.Id, assigned.Roles[0].Id)
		registered, err := pricing.Get(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "Pricing page", registered.Name)
		assert.Equal(t, "folder", registered.Parent.Type)
		assert.Equal(t, w.folder, registered.Parent.Id)
		sink.WaitFor("resource.role_assignment.created", map[string]string{
			"organization_id": w.org.Id, "resource_type": "document", "resource_id": pricing.ID(),
			"product_user_id": dan, "role_id": w.documentViewer.Id,
		})
		// Dan's role is on the document; the world's member reaches it only
		// through the folder, so this also proves the stored parent.
		requireAllowed(t, w.access.User(danExternalID).OnResource(w.org.Id, "document", pricing.ID()).
			Can(documentView).Decide())
		requireAllowed(t, w.asMember("document", pricing.ID()).Can(documentView).Decide())
	})

	t.Run("A put that assigns on a registered document renames and moves it, then sets the role", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, _ := w.newMember(t, w.orgMember.Id)
		brief := w.documentHandle(w.product.SDKClient(), w.document)

		_, err := brief.Put("Brief v2").Under("folder", w.subfolder).Assign(t.Context(), dan, w.documentViewer.Id)

		require.NoError(t, err)
		moved, err := brief.Get(t.Context())
		require.NoError(t, err)
		assert.Equal(t, "Brief v2", moved.Name)
		assert.Equal(t, w.subfolder, moved.Parent.Id)
		held, err := brief.Members().Get(t.Context(), dan)
		require.NoError(t, err)
		assert.Equal(t, w.documentViewer.Id, held.Roles[0].Id)
	})

	t.Run("A parent that is not registered is a 409 conflict, and nothing is written", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, _ := w.newMember(t, w.orgMember.Id)
		pricing := w.documentHandle(w.product.SDKClient(), resourceID())

		_, err := pricing.Put("Pricing page").Under("folder", resourceID()).
			Assign(t.Context(), dan, w.documentViewer.Id)

		requireSDKError(t, err, anchorsdk.ErrConflict, "PARENT_NOT_REGISTERED")
		_, err = pricing.Get(t.Context())
		assert.ErrorIs(t, err, anchorsdk.ErrNotFound)
	})

	t.Run("A role the document can't hold is a 400, and the document stays unregistered", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, _ := w.newMember(t, w.orgMember.Id)
		pricing := w.documentHandle(w.product.SDKClient(), resourceID())

		_, err := pricing.Put("Pricing page").Under("folder", w.folder).Assign(t.Context(), dan, w.folderViewer.Id)

		requireSDKError(t, err, anchorsdk.ErrInvalid, "ROLE_SCOPE_MISMATCH")
		_, err = pricing.Get(t.Context())
		assert.ErrorIs(t, err, anchorsdk.ErrNotFound)
	})

	t.Run("Assigning without the resource block on a document that is not registered is a 404", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, _ := w.newMember(t, w.orgMember.Id)

		_, err := w.documentHandle(w.product.SDKClient(), resourceID()).Members().
			Assign(t.Context(), dan, w.documentViewer.Id)

		requireSDKError(t, err, anchorsdk.ErrNotFound, "RESOURCE_NOT_FOUND")
	})

	t.Run("A key without resource:write can't register on assign, and can still assign", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, _ := w.newMember(t, w.orgMember.Id)
		memberOnly := w.product.SDKClient("resource_member:write", "resource:read")
		pricing := w.documentHandle(memberOnly, resourceID())

		_, err := pricing.Put("Pricing page").Under("folder", w.folder).Assign(t.Context(), dan, w.documentViewer.Id)

		metadata := requireSDKError(t, err, anchorsdk.ErrForbidden, "PRODUCT_API_KEY_INSUFFICIENT_PERMISSIONS")
		assert.Equal(t, []any{"resource:write"}, metadata["required_scopes"])
		_, err = pricing.Get(t.Context())
		require.ErrorIs(t, err, anchorsdk.ErrNotFound)
		_, err = w.documentHandle(memberOnly, w.document).Members().Assign(t.Context(), dan, w.documentViewer.Id)
		require.NoError(t, err)
	})
}

func TestResourceMemberPutRegisteringRacesAParentDelete(t *testing.T) {
	t.Parallel()
	t.Run(
		"Registering under a folder while it is deleted either lands and goes with the folder, or is a 409",
		func(t *testing.T) {
			t.Parallel()
			w := givenResourceAccessWorld(t)
			dan, _ := w.newMember(t, w.orgMember.Id)
			org := w.product.SDKClient().Organization(w.org.Id)

			for range 5 {
				folder := org.Resource("folder", resourceID())
				_, err := folder.Put("Scratch").Under("project", w.project).Do(t.Context())
				require.NoError(t, err)
				document := org.Resource("document", resourceID())
				var (
					wg                   sync.WaitGroup
					assignErr, deleteErr error
				)
				wg.Go(func() {
					_, assignErr = document.Put("Draft").Under("folder", folder.ID()).
						Assign(t.Context(), dan, w.documentViewer.Id)
				})
				wg.Go(func() { deleteErr = folder.DeleteTree(t.Context()) })
				wg.Wait()

				require.NoError(t, deleteErr)
				if assignErr != nil {
					requireSDKError(t, assignErr, anchorsdk.ErrConflict, "PARENT_NOT_REGISTERED")
				}
				_, err = document.Get(t.Context())
				assert.ErrorIs(t, err, anchorsdk.ErrNotFound)
			}
		},
	)
}
