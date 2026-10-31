package ct_test

import (
	"net/http"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// putMemberRegistering assigns the role on a document and sends the resource
// block, so a document that is not registered is registered under parent.
func (w resourceAccessWorld) putMemberRegistering(
	t *testing.T,
	client *ct.ClientWithResponses,
	document, productUserID, roleID, name string,
	parent ct.ResourceParentRef,
) *ct.PutResourceMemberResponse {
	t.Helper()
	resp, err := client.PutResourceMemberWithResponse(t.Context(), w.product.ProductID, w.org.Id, "document",
		document, productUserID, ct.PutResourceMemberJSONRequestBody{
			RoleIds:  []string{roleID},
			Resource: &ct.ResourcePutRequest{Name: name, Parent: parent},
		})
	require.NoError(t, err)
	return resp
}

func TestResourceMemberPutRegistersTheResource(t *testing.T) {
	t.Parallel()

	t.Run("One call registers a document under its folder and gives the member a role on it", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, danExternalID := w.newMember(t, w.orgMember.Id)
		sink := w.product.CaptureEvents()
		pricing := resourceID()

		resp := w.putMemberRegistering(t, w.product.AllScopeAPIKeyClient(), pricing, dan, w.documentViewer.Id,
			"Pricing page", ct.ResourceParentRef{Type: "folder", Id: w.folder})

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.Equal(t, w.documentViewer.Id, resp.JSON200.Roles[0].Id)
		registered := w.resources.Get("document", pricing)
		assert.Equal(t, "Pricing page", registered.Name)
		assert.Equal(t, ct.ResourceParentRef{Type: "folder", Id: w.folder}, registered.Parent)
		sink.WaitFor("resource.role_assignment.created", map[string]string{
			"organization_id": w.org.Id, "resource_type": "document", "resource_id": pricing,
			"product_user_id": dan, "role_id": w.documentViewer.Id,
		})
		// Dan's role is on the document; the world's member reaches it only
		// through the folder, so this also proves the stored parent.
		requireAllowed(t, w.access.User(danExternalID).OnResource(w.org.Id, "document", pricing).
			Can(documentView).Decide())
		requireAllowed(t, w.asMember("document", pricing).Can(documentView).Decide())
	})

	t.Run("A block on a registered document renames and moves it, then sets the role", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, _ := w.newMember(t, w.orgMember.Id)

		resp := w.putMemberRegistering(t, w.product.AllScopeAPIKeyClient(), w.document, dan, w.documentViewer.Id,
			"Brief v2", ct.ResourceParentRef{Type: "folder", Id: w.subfolder})

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		moved := w.resources.Get("document", w.document)
		assert.Equal(t, "Brief v2", moved.Name)
		assert.Equal(t, ct.ResourceParentRef{Type: "folder", Id: w.subfolder}, moved.Parent)
		assert.Equal(t, w.documentViewer.Id, w.members("document", w.document).Get(dan).Roles[0].Id)
	})

	t.Run("A parent that is not registered answers 409 and writes nothing", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, _ := w.newMember(t, w.orgMember.Id)
		pricing := resourceID()

		resp := w.putMemberRegistering(t, w.product.AllScopeAPIKeyClient(), pricing, dan, w.documentViewer.Id,
			"Pricing page", ct.ResourceParentRef{Type: "folder", Id: resourceID()})

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "PARENT_NOT_REGISTERED")
		got := w.resources.GetRaw("document", pricing)
		assert.Equal(t, http.StatusNotFound, got.StatusCode(), string(got.Body))
	})

	t.Run("A role the document can't hold answers 400 and leaves the document unregistered", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, _ := w.newMember(t, w.orgMember.Id)
		pricing := resourceID()

		resp := w.putMemberRegistering(t, w.product.AllScopeAPIKeyClient(), pricing, dan, w.folderViewer.Id,
			"Pricing page", ct.ResourceParentRef{Type: "folder", Id: w.folder})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_SCOPE_MISMATCH")
		got := w.resources.GetRaw("document", pricing)
		assert.Equal(t, http.StatusNotFound, got.StatusCode(), string(got.Body))
	})

	t.Run("Without the block, a document that is not registered answers 404", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, _ := w.newMember(t, w.orgMember.Id)

		resp := w.members("document", resourceID()).PutRaw(dan, w.documentViewer.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})

	t.Run("A key without resource:write can't send the block, and can still assign without it", func(t *testing.T) {
		t.Parallel()
		w := givenResourceAccessWorld(t)
		dan, _ := w.newMember(t, w.orgMember.Id)
		memberOnly, _ := w.product.CreateAPIKeyClientWithScopes([]string{"resource_member:write"})
		pricing := resourceID()

		resp := w.putMemberRegistering(t, memberOnly, pricing, dan, w.documentViewer.Id,
			"Pricing page", ct.ResourceParentRef{Type: "folder", Id: w.folder})

		metadata := requireAPIError(
			t,
			resp,
			resp.Body,
			http.StatusForbidden,
			"PRODUCT_API_KEY_INSUFFICIENT_PERMISSIONS",
		)
		assert.Equal(t, []any{"resource:write"}, metadata["required_scopes"])
		got := w.resources.GetRaw("document", pricing)
		assert.Equal(t, http.StatusNotFound, got.StatusCode(), string(got.Body))
		plain := w.product.ResourceMembers(w.org.Id, "document", w.document, "resource_member:write").
			PutRaw(dan, w.documentViewer.Id)
		assert.Equal(t, http.StatusOK, plain.StatusCode(), string(plain.Body))
	})
}

func TestResourceMemberPutRegisteringRacesAParentDelete(t *testing.T) {
	t.Parallel()
	t.Run(
		"Registering under a folder while it is deleted either lands and goes with the folder, or answers 409",
		func(t *testing.T) {
			t.Parallel()
			w := givenResourceAccessWorld(t)
			dan, _ := w.newMember(t, w.orgMember.Id)
			client := w.product.AllScopeAPIKeyClient()

			for range 5 {
				folder, document := resourceID(), resourceID()
				w.resources.Put("folder", folder, "Scratch", ct.ResourceParentRef{Type: "project", Id: w.project})
				var (
					wg                   sync.WaitGroup
					assigned             *ct.PutResourceMemberResponse
					deleted              *ct.DeleteResourceResponse
					assignErr, deleteErr error
				)
				wg.Go(func() {
					assigned, assignErr = client.PutResourceMemberWithResponse(
						t.Context(),
						w.product.ProductID,
						w.org.Id,
						"document",
						document,
						dan,
						ct.PutResourceMemberJSONRequestBody{
							RoleIds: []string{w.documentViewer.Id},
							Resource: &ct.ResourcePutRequest{
								Name: "Draft", Parent: ct.ResourceParentRef{Type: "folder", Id: folder},
							},
						},
					)
				})
				wg.Go(func() {
					deleted, deleteErr = client.DeleteResourceWithResponse(t.Context(), w.product.ProductID, w.org.Id,
						"folder", folder, &ct.DeleteResourceParams{Cascade: new(true)})
				})
				wg.Wait()

				require.NoError(t, assignErr)
				require.NoError(t, deleteErr)
				require.Equal(t, http.StatusNoContent, deleted.StatusCode(), string(deleted.Body))
				if assigned.StatusCode() != http.StatusOK {
					requireAPIError(t, assigned, assigned.Body, http.StatusConflict, "PARENT_NOT_REGISTERED")
				}
				got := w.resources.GetRaw("document", document)
				assert.Equal(t, http.StatusNotFound, got.StatusCode(), string(got.Body))
			}
		},
	)
}
