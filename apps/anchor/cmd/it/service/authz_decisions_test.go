package service_test

import (
	"testing"

	"github.com/google/uuid"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/model"
)

func TestAuthzDecisions(t *testing.T) {
	openFGA(t)

	t.Run("A decision comes from the store, and a reconcile repairs a deleted tuple", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, userID := givenMemberWithExternalID(t, w)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		require.True(t, evaluate(t, userRequest(w, externalID)).Allowed)

		deleteTuple(t, w.productID, &openfgav1.TupleKeyWithoutCondition{
			Object:   model.Assignment(w.organizationID, userID, w.templateID),
			Relation: model.RelationAssignee,
			User:     model.User(userID),
		})

		denied := evaluate(t, userRequest(w, externalID))
		assert.False(t, denied.Allowed)
		assert.Equal(t, accessservice.ReasonMissingPermission, denied.Items[0].Reason)

		require.NoError(t, AuthzSyncer.Reconcile(t.Context(), w.productID))
		assert.True(t, evaluate(t, userRequest(w, externalID)).Allowed)
	})

	t.Run("The first decision for a Product nothing synced yet builds its store", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, _ := givenMemberWithExternalID(t, w)

		decision := evaluate(t, userRequest(w, externalID))

		assert.True(t, decision.Allowed)
		found, err := AuthzRepo.FindStore(t.Context(), w.productID)
		require.NoError(t, err)
		assert.NotEmpty(t, found.Value().ModelID)
	})

	t.Run("A permission the Product doesn't declare is not held", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, _ := givenMemberWithExternalID(t, w)
		request := userRequest(w, externalID)
		request.Actions = []string{w.permission, "unknown:" + uuid.NewString()}

		decision := evaluate(t, request)

		assert.True(t, decision.Items[0].Allowed)
		assert.False(t, decision.Items[1].Allowed)
		assert.Equal(t, accessservice.ReasonMissingPermission, decision.Items[1].Reason)
	})

	t.Run("A decision on more actions than one BatchCheck takes answers each of them", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, _ := givenMemberWithExternalID(t, w)
		request := userRequest(w, externalID)
		request.Actions = nil
		for range 55 {
			request.Actions = append(request.Actions, "unknown:"+uuid.NewString())
		}
		request.Actions = append(request.Actions, w.permission)

		decision := evaluate(t, request)

		require.Len(t, decision.Items, 56)
		assert.False(t, decision.Items[0].Allowed)
		assert.True(t, decision.Items[55].Allowed)
	})
}

func evaluate(t *testing.T, request accessservice.Request) accessservice.Decision {
	t.Helper()
	decision, err := Evaluator.Evaluate(t.Context(), request)
	require.NoError(t, err)
	return decision
}

func deleteTuple(t *testing.T, productID string, tuple *openfgav1.TupleKeyWithoutCondition) {
	t.Helper()
	found, err := AuthzRepo.FindStore(t.Context(), productID)
	require.NoError(t, err)
	_, err = openFGA(t).Write(t.Context(), &openfgav1.WriteRequest{
		StoreId: found.Value().StoreID, AuthorizationModelId: found.Value().ModelID,
		Deletes: &openfgav1.WriteRequestDeletes{TupleKeys: []*openfgav1.TupleKeyWithoutCondition{tuple}},
	})
	require.NoError(t, err)
}

// givenMemberWithExternalID adds, straight into the table, a member the
// evaluator can name; no mark is left, so the store learns of it only on a
// sync.
func givenMemberWithExternalID(t *testing.T, w syncWorld) (string, string) {
	t.Helper()
	userID := givenMember(t, w, w.organizationID, w.templateID)
	externalID := "user_" + userID
	_, err := TestDB.ExecContext(t.Context(),
		`UPDATE product_users SET external_id = $1 WHERE id = $2`, externalID, userID)
	require.NoError(t, err)
	return externalID, userID
}

func userRequest(w syncWorld, externalID string) accessservice.Request {
	return accessservice.Request{
		ProductID: w.productID,
		Subject:   accessservice.Subject{Type: accessservice.SubjectUser, ID: externalID},
		Resource:  accessservice.Resource{Type: accessservice.ResourceOrganization, ID: w.organizationID},
		Actions:   []string{w.permission},
	}
}
