package ct_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

func TestConcurrentResourceMemberWrites(t *testing.T) {
	t.Parallel()

	t.Run("Concurrent assignments for one member all succeed and leave one role", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		members := w.folderMembers()
		for range concurrentRounds {
			member := w.newMember(t)
			var wg sync.WaitGroup
			statuses := make([]int, concurrentWriters)
			for i := range concurrentWriters {
				role := w.editor.Id
				if i%2 == 0 {
					role = w.viewer.Id
				}
				wg.Go(func() { statuses[i] = members.PutRaw(member, role).StatusCode() })
			}
			wg.Wait()

			for _, status := range statuses {
				require.Equal(t, http.StatusOK, status)
			}
			assert.Equal(t, 1, countAssignments(t, w.org.Id, member))
			assert.Len(t, members.Get(member).Roles, 1)
		}
	})

	t.Run("An assignment racing the member's removal never outlives the membership", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		client := w.product.AllScopeAPIKeyClient()
		sink := w.product.CaptureEvents()
		for range concurrentRounds {
			member := w.newMember(t)
			var wg sync.WaitGroup
			var assigned, removed int
			wg.Go(func() { assigned = w.folderMembers().PutRaw(member, w.editor.Id).StatusCode() })
			wg.Go(func() {
				resp, err := client.RemoveOrganizationMemberWithResponse(
					context.Background(), w.product.ProductID, w.org.Id, member)
				if assert.NoError(t, err) {
					removed = resp.StatusCode()
				}
			})
			wg.Wait()

			assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, assigned)
			assert.Equal(t, http.StatusNoContent, removed)
			assert.Zero(t, countAssignments(t, w.org.Id, member))
			if assigned == http.StatusOK {
				sink.WaitFor("resource.role_assignment.deleted",
					w.assignmentEvent(member, "folder", w.folder, w.editor.Id))
			}
		}
		requireAssignmentEventsBalance(t, sink)
	})

	t.Run("An assignment racing its resource's cascade delete never outlives the resource", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		sink := w.product.CaptureEvents()
		for range concurrentRounds {
			folder := resourceID()
			w.resources.Put("folder", folder, "Design", ct.ResourceParentRef{Type: "project", Id: w.project})
			var wg sync.WaitGroup
			var assigned, deleted int
			wg.Go(func() {
				assigned = w.product.ResourceMembers(w.org.Id, "folder", folder).
					PutRaw(w.member, w.editor.Id).
					StatusCode()
			})
			wg.Go(func() { deleted = w.resources.DeleteRaw("folder", folder, true).StatusCode() })
			wg.Wait()

			assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, assigned)
			assert.Equal(t, http.StatusNoContent, deleted)
			if assigned == http.StatusOK {
				sink.WaitFor(
					"resource.role_assignment.deleted",
					w.assignmentEvent(w.member, "folder", folder, w.editor.Id),
				)
			}
		}
		assert.Zero(t, countAssignments(t, w.org.Id, w.member))
		requireAssignmentEventsBalance(t, sink)
	})

	t.Run("Removing a member's role racing the member's removal reports each removed role once", func(t *testing.T) {
		t.Parallel()
		w := givenMemberWorld(t)
		client := w.product.AllScopeAPIKeyClient()
		sink := w.product.CaptureEvents()
		for range concurrentRounds {
			member := w.newMember(t)
			w.folderMembers().Put(member, w.editor.Id)
			var wg sync.WaitGroup
			var unassigned int
			wg.Go(func() { unassigned = w.folderMembers().DeleteRaw(member).StatusCode() })
			wg.Go(func() {
				resp, err := client.RemoveOrganizationMemberWithResponse(
					context.Background(), w.product.ProductID, w.org.Id, member)
				if assert.NoError(t, err) {
					assert.Equal(t, http.StatusNoContent, resp.StatusCode())
				}
			})
			wg.Wait()

			assert.Contains(t, []int{http.StatusNoContent, http.StatusNotFound}, unassigned)
			sink.WaitFor("resource.role_assignment.deleted", w.assignmentEvent(member, "folder", w.folder, w.editor.Id))
		}
		requireAssignmentEventsBalance(t, sink)
	})
}

// Not parallel: lock-timing assertion.
func TestRoleTemplateDeleteDuringAnAssignment(t *testing.T) {
	t.Run("A role assigned on a resource while its template is deleted is never held", func(t *testing.T) {
		w := givenMemberWorld(t)
		invitationID := ids.MustNew("orginv")
		_, err := testDB.ExecContext(t.Context(), `INSERT INTO organization_invitations
			(id, product_id, organization_id, email, product_role_id, expires_at)
			VALUES ($1, $2, $3, $4, $5, now() - interval '1 day')`,
			invitationID, w.product.ProductID, w.org.Id, ids.MustNew("ct")+"@example.com", w.editor.Id)
		require.NoError(t, err)
		releaseInvitation := holdRowForUpdate(t, "organization_invitations", invitationID)

		deleted := make(chan int, 1)
		go func() {
			resp, deleteErr := w.product.OwnerAuthenticatedClient().DeleteProductRoleWithResponse(
				context.Background(), w.product.ProductID, w.editor.Id)
			if assert.NoError(t, deleteErr) {
				deleted <- resp.StatusCode()
			}
		}()
		waitUntilStatementWaitsOnALock(t, "DELETE", "organization_invitations")
		assigned := make(chan *ct.PutResourceMemberResponse, 1)
		go func() { assigned <- w.folderMembers().PutRaw(w.member, w.editor.Id) }()
		waitUntilStatementWaitsOnALock(t, "SELECT", "product_roles")
		releaseInvitation()

		assert.Equal(t, http.StatusNoContent, <-deleted)
		resp := <-assigned
		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_NOT_FOUND_IN_REQUEST")
		assert.Zero(t, countAssignments(t, w.org.Id, w.member))
	})
}

// requireAssignmentEventsBalance waits until the endpoint received one
// deleted event per created event: every role an assignment gave is reported
// gone exactly once.
func requireAssignmentEventsBalance(t *testing.T, sink *itdsl.EventSink) {
	t.Helper()
	require.Eventually(t, func() bool {
		return sink.Count("resource.role_assignment.created") ==
			sink.Count("resource.role_assignment.deleted")
	}, mirrorWait, mirrorPoll, "created and deleted role assignment events differ")
	assert.Zero(t, sink.Count("resource.role_assignment.updated"))
}
