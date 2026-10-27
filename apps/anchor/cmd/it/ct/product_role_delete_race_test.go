package ct_test

import (
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A role delete and a member added on that role race on the role's row: the
// delete either sees the member and answers 409, or the add waits for the
// delete and fails. A 204 beside a 201 would mean the delete's cascade removed
// a member the add had just reported.
func TestConcurrentRoleDeleteAndMemberAdd(t *testing.T) {
	t.Parallel()
	t.Run("A role delete racing a member add never deletes the member it added", func(t *testing.T) {
		t.Parallel()
		const rounds = 20
		for range rounds {
			product := createTestProductContext(t)
			org := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
			managed := createScopedRole(t, product, "organization")
			user := createDSLProductUser(t, product)
			client := product.AllScopeAPIKeyClient()

			var deleteStatus, addStatus int
			var addBody []byte
			var wg sync.WaitGroup
			start := make(chan struct{})
			wg.Go(func() {
				<-start
				resp, err := client.DeleteProductRoleWithResponse(t.Context(), product.ProductID, managed.Id)
				if assert.NoError(t, err) {
					deleteStatus = resp.StatusCode()
				}
			})
			wg.Go(func() {
				<-start
				resp, err := client.AddOrganizationMemberWithResponse(t.Context(), product.ProductID, org.Id,
					ct.AddOrganizationMemberJSONRequestBody{ProductUserId: user.ID, RoleId: new(managed.Id)})
				if assert.NoError(t, err) {
					addStatus, addBody = resp.StatusCode(), resp.Body
				}
			})
			close(start)
			wg.Wait()
			member, err := client.GetOrganizationMemberWithResponse(
				t.Context(), product.ProductID, org.Id, user.ID, nil,
			)
			require.NoError(t, err)

			switch deleteStatus {
			case http.StatusConflict:
				assert.Equal(t, http.StatusCreated, addStatus)
				assert.Equal(t, http.StatusOK, member.StatusCode(), "the delete refused, so the member stays")
			case http.StatusNoContent:
				assert.NotEqual(t, http.StatusCreated, addStatus, "the delete removed the member the add reported")
				assert.Equal(t, http.StatusBadRequest, addStatus, string(addBody))
				assert.Contains(t, string(addBody), "ROLE_NOT_FOUND_IN_REQUEST")
				assert.Equal(t, http.StatusNotFound, member.StatusCode())
			default:
				t.Errorf("delete answered %d, add %d", deleteStatus, addStatus)
			}
		}
	})
}
