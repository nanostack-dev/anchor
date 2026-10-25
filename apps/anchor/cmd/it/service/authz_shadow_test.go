package service_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accessservice "anchor/internal/access/service"
)

func TestAuthzShadow(t *testing.T) {
	openFGA(t)

	t.Run(
		"A decision the store agrees with counts no difference, and the evaluator's answer stands",
		func(t *testing.T) {
			w := givenSyncWorld(t)
			externalID := givenMemberWithExternalID(t, w)
			require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
			ShadowStats.Settle()
			compared, mismatches := ShadowStats.Compared(), ShadowStats.Mismatches()

			decision, err := Evaluator.Evaluate(t.Context(), userRequest(w, externalID))

			require.NoError(t, err)
			assert.True(t, decision.Allowed)
			ShadowStats.Settle()
			assert.Equal(t, compared+1, ShadowStats.Compared())
			assert.Equal(t, mismatches, ShadowStats.Mismatches())
		},
	)

	t.Run("A decision the store disagrees with is counted, and the evaluator's answer stands", func(t *testing.T) {
		w := givenSyncWorld(t)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		externalID := givenMemberWithExternalID(t, w)
		ShadowStats.Settle()
		before := ShadowStats.Mismatches()

		decision, err := Evaluator.Evaluate(t.Context(), userRequest(w, externalID))

		require.NoError(t, err)
		assert.True(t, decision.Allowed)
		ShadowStats.Settle()
		assert.Equal(t, before+1, ShadowStats.Mismatches())
	})

	t.Run("More actions than one BatchCheck takes are all compared, and an undeclared one agrees", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID := givenMemberWithExternalID(t, w)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		request := userRequest(w, externalID)
		for range 55 {
			request.Actions = append(request.Actions, "unknown:"+uuid.NewString())
		}
		ShadowStats.Settle()
		compared, mismatches, failed := ShadowStats.Compared(), ShadowStats.Mismatches(), ShadowStats.Failed()

		_, err := Evaluator.Evaluate(t.Context(), request)

		require.NoError(t, err)
		ShadowStats.Settle()
		assert.Equal(t, compared+56, ShadowStats.Compared())
		assert.Equal(t, mismatches, ShadowStats.Mismatches())
		assert.Equal(t, failed, ShadowStats.Failed())
	})
}

// givenMemberWithExternalID adds, straight into the table, a member the
// evaluator can name; no mark is left, so the store learns of it only on a
// sync.
func givenMemberWithExternalID(t *testing.T, w syncWorld) string {
	t.Helper()
	userID := givenMember(t, w, w.organizationID, w.templateID)
	externalID := "user_" + userID
	_, err := TestDB.ExecContext(
		t.Context(),
		`UPDATE product_users SET external_id = $1 WHERE id = $2`,
		externalID,
		userID,
	)
	require.NoError(t, err)
	return externalID
}

func userRequest(w syncWorld, externalID string) accessservice.Request {
	return accessservice.Request{
		ProductID: w.productID,
		Subject:   accessservice.Subject{Type: accessservice.SubjectUser, ID: externalID},
		Resource:  accessservice.Resource{Type: accessservice.ResourceOrganization, ID: w.organizationID},
		Actions:   []string{w.permission},
	}
}
