package ct_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// productUserService.Delete calls roleassignment ReleaseUser before it deletes
// the user, and ReleaseUser lists the assignments by product user ID alone,
// with no product filter. It then emits one resource.role_assignment.deleted
// per assignment under the caller's product. The delete itself is product
// scoped, finds nothing and answers 204, so the transaction commits and the
// events reach the caller's endpoint: another product's organization IDs,
// resource identifiers, role IDs and memberships leak to any holder of a
// product_user:delete key that knows one of its product user IDs.
func TestSecurityProductUserDeleteAcrossProducts(t *testing.T) {
	t.Parallel()

	t.Run("Deleting another product's user through your own product delivers none of its resource roles",
		func(t *testing.T) {
			t.Parallel()
			victim := givenMemberWorld(t)
			victim.folderMembers().Put(victim.member, victim.editor.Id)
			attacker := createTestProductContext(t)
			sink := attacker.CaptureEvents()

			resp, err := attacker.AllScopeAPIKeyClient().DeleteProductUserWithResponse(
				t.Context(), attacker.ProductID, victim.member)
			require.NoError(t, err)
			require.Contains(t, []int{http.StatusNoContent, http.StatusNotFound}, resp.StatusCode(), string(resp.Body))

			assert.Never(t, func() bool {
				return len(sink.Events("resource.role_assignment.deleted")) > 0
			}, 5*time.Second, 100*time.Millisecond)
			for _, leaked := range sink.Events("resource.role_assignment.deleted") {
				t.Errorf("victim's assignment leaked to the attacker's endpoint: %s", leaked.Data)
			}
			assert.Equal(t, 1, countAssignments(t, victim.org.Id, victim.member))
		})
}
