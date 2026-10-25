package ct_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestDeletesReleaseResourceRoles(t *testing.T) {
	t.Parallel()

	t.Run("Deleting the organization emits a deleted event for every role held on its resources", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		other := w.newMember(t)
		w.folderMembers().Put(w.member, w.editor.Id)
		w.projectMembers().Put(other, w.projectAdmin.Id)
		sink := w.product.CaptureEvents()

		resp, err := w.product.AllScopeAPIKeyClient().DeleteProductOrganizationWithResponse(
			t.Context(), w.product.ProductID, w.org.Id)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

		sink.WaitFor("resource.role_assignment.deleted", w.assignmentEvent(w.member, "folder", w.folder, w.editor.Id))
		sink.WaitFor("resource.role_assignment.deleted",
			w.assignmentEvent(other, "project", w.project, w.projectAdmin.Id))
		assert.Zero(t, countAssignments(t, w.org.Id, w.member))
	})

	t.Run("Deleting the product user emits a deleted event for every role it held, in every organization",
		func(t *testing.T) {
			t.Parallel()
			w := givenMemberWorld(t)
			w.folderMembers().Put(w.member, w.editor.Id)
			w.projectMembers().Put(w.member, w.projectAdmin.Id)
			secondOrg := w.product.CreateOrganization(t, "Org-"+resourceID(), nil)
			addMember(t, w.product, secondOrg.Id, w.member, w.orgMember.Id)
			secondResources := w.product.Resources(secondOrg.Id)
			elsewhere := resourceID()
			secondResources.Put("project", elsewhere, "Elsewhere", secondResources.Top())
			w.product.ResourceMembers(secondOrg.Id, "project", elsewhere).Put(w.member, w.projectAdmin.Id)
			sink := w.product.CaptureEvents()

			resp, err := w.product.AllScopeAPIKeyClient().DeleteProductUserWithResponse(
				t.Context(), w.product.ProductID, w.member)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))

			sink.WaitFor(
				"resource.role_assignment.deleted",
				w.assignmentEvent(w.member, "folder", w.folder, w.editor.Id),
			)
			sink.WaitFor("resource.role_assignment.deleted",
				w.assignmentEvent(w.member, "project", w.project, w.projectAdmin.Id))
			sink.WaitFor("resource.role_assignment.deleted", map[string]string{
				"organization_id": secondOrg.Id, "resource_type": "project", "resource_id": elsewhere,
				"product_user_id": w.member, "role_id": w.projectAdmin.Id,
			})
			assert.Zero(t, countAssignments(t, w.org.Id, w.member))
			assert.Zero(t, countAssignments(t, secondOrg.Id, w.member))
		})

	t.Run("A Clerk user deletion emits a deleted event for every role the user held", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		createActiveClerkIntegrationInstance(t, w.product)
		externalID := "user_clerk_" + itshared.Faker.UUID().V4()
		created := sendClerkWebhook(t, w.product.ProductID,
			clerkUserCreatedPayload(t, externalID, itshared.Faker.Internet().Email(), "Held", "Roles"),
			clerkTestWebhookSecret)
		require.Equal(t, http.StatusOK, created.StatusCode())
		var userID string
		require.Eventually(t, func() bool {
			for _, item := range searchProductUsers(t, w.product).JSON200.Items {
				if item.ExternalId != nil && *item.ExternalId == externalID {
					userID = item.Id
					return true
				}
			}
			return false
		}, 10*time.Second, 200*time.Millisecond)
		addMember(t, w.product, w.org.Id, userID, w.orgMember.Id)
		w.folderMembers().Put(userID, w.editor.Id)
		sink := w.product.CaptureEvents()

		deleted := sendClerkWebhook(
			t,
			w.product.ProductID,
			clerkUserDeletedPayload(t, externalID),
			clerkTestWebhookSecret,
		)
		require.Equal(t, http.StatusOK, deleted.StatusCode())

		sink.WaitFor("resource.role_assignment.deleted", w.assignmentEvent(userID, "folder", w.folder, w.editor.Id))
		assert.Zero(t, countAssignments(t, w.org.Id, userID))
	})
}
