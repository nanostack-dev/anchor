package service_test

import (
	"testing"

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
			before := ShadowStats.Mismatches()

			decision, err := Evaluator.Evaluate(t.Context(), userRequest(w, externalID))

			require.NoError(t, err)
			assert.True(t, decision.Allowed)
			assert.Equal(t, before, ShadowStats.Mismatches())
		},
	)

	t.Run("A decision the store disagrees with is counted, and the evaluator's answer stands", func(t *testing.T) {
		w := givenSyncWorld(t)
		require.NoError(t, AuthzSyncer.SyncOrganization(t.Context(), w.productID, w.organizationID))
		externalID := givenMemberWithExternalID(t, w)
		before := ShadowStats.Mismatches()

		decision, err := Evaluator.Evaluate(t.Context(), userRequest(w, externalID))

		require.NoError(t, err)
		assert.True(t, decision.Allowed)
		assert.Equal(t, before+1, ShadowStats.Mismatches())
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
