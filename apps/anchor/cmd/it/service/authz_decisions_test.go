package service_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/decision"
	authzengine "anchor/internal/authz/engine"
	"anchor/internal/authz/fga"
	"anchor/internal/authz/model"
	authzstore "anchor/internal/authz/store"
)

func TestAuthzDecisions(t *testing.T) {
	openFGA(t)

	t.Run("A decision comes from the store, and a reconcile repairs a deleted tuple", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, userID := givenMemberWithExternalID(t, w)
		rebuild(t, w.productID)
		require.True(t, evaluate(t, userRequest(w, externalID)).Allowed)

		deleteTuple(t, w.productID, fga.Tuple{
			Object:   model.Assignment(w.organizationID, w.managedID),
			Relation: model.RelationAssignee,
			User:     model.User(userID),
		})

		denied := evaluate(t, userRequest(w, externalID))
		assert.False(t, denied.Allowed)
		assert.Equal(t, accessservice.ReasonMissingPermission, denied.Items[0].Reason)

		rebuild(t, w.productID)
		assert.True(t, evaluate(t, userRequest(w, externalID)).Allowed)
	})

	t.Run("The first decision for a Product nothing synced yet builds its store", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, _ := givenMemberWithExternalID(t, w)

		decision := evaluate(t, userRequest(w, externalID))

		assert.True(t, decision.Allowed)
		found, err := AuthzStores.Open(t.Context(), w.productID)
		require.NoError(t, err)
		opened, ok := found.Get()
		require.True(t, ok)
		assert.True(t, opened.Rebuilt)
	})

	t.Run("A store some Organizations were synced into is rebuilt before it answers", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, _ := givenMemberWithExternalID(t, w)
		other := givenOrganization(t, w.productID)
		syncKey(t, w.productID, assignments, other)

		assert.True(t, evaluate(t, userRequest(w, externalID)).Allowed)
	})

	t.Run("Concurrent first decisions for a Product all get its answer", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, _ := givenMemberWithExternalID(t, w)
		var group errgroup.Group
		for range 10 {
			group.Go(func() error {
				decision, err := Evaluator.Evaluate(t.Context(), userRequest(w, externalID))
				if err == nil && !decision.Allowed {
					err = errors.New("denied")
				}
				return err
			})
		}

		require.NoError(t, group.Wait())
	})

	t.Run("The first decision after a release that compiles a new model writes it", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, _ := givenMemberWithExternalID(t, w)
		rebuild(t, w.productID)
		var before string
		require.NoError(t, TestDB.QueryRowContext(t.Context(),
			`UPDATE authz_stores SET model_hash = 'previous release' WHERE product_id = $1 RETURNING model_id`,
			w.productID).Scan(&before))
		checker := decision.NewChecker(AuthzStores, AuthzSyncer)
		evaluator := accessservice.NewEvaluator(ProductUserRepo, MembershipRepo, OrganizationRepo,
			OrgAPIKeyService, checker, ResourceTypeService, ResourceRepo, RoleAssignmentRepo, TestLogger)

		decided, err := evaluator.Evaluate(t.Context(), userRequest(w, externalID))

		require.NoError(t, err)
		assert.True(t, decided.Allowed)
		var after, hash string
		require.NoError(t, TestDB.QueryRowContext(t.Context(),
			`SELECT model_id, model_hash FROM authz_stores WHERE product_id = $1`, w.productID).Scan(&after, &hash))
		assert.NotEqual(t, before, after, "the store follows a newly written model")
		assert.NotEqual(t, "previous release", hash)
	})

	t.Run("An engine that isn't ready answers 503", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, _ := givenMemberWithExternalID(t, w)
		checker := decision.NewChecker(authzstore.New(TestDB, notReady{}, TestLogger), AuthzSyncer)
		evaluator := accessservice.NewEvaluator(ProductUserRepo, MembershipRepo, OrganizationRepo,
			OrgAPIKeyService, checker, ResourceTypeService, ResourceRepo, RoleAssignmentRepo, TestLogger)

		_, err := evaluator.Evaluate(t.Context(), userRequest(w, externalID))

		require.ErrorIs(t, err, accessservice.ErrDecisionsUnavailable)
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

type notReady struct{}

func (notReady) Get() (authzengine.Engine, error) { return nil, authzengine.ErrNotReady }

func evaluate(t *testing.T, request accessservice.Request) accessservice.Decision {
	t.Helper()
	decision, err := Evaluator.Evaluate(t.Context(), request)
	require.NoError(t, err)
	return decision
}

func deleteTuple(t *testing.T, productID string, tuple fga.Tuple) {
	t.Helper()
	found, err := AuthzStores.Open(t.Context(), productID)
	require.NoError(t, err)
	opened, ok := found.Get()
	require.True(t, ok)
	require.NoError(t, opened.Store.Apply(t.Context(), fga.Changes{Deletes: []fga.Tuple{tuple}}))
}

// givenMemberWithExternalID adds, straight into the table, a member the
// evaluator can name; no mark is left, so the store learns of it only on a
// sync.
func givenMemberWithExternalID(t *testing.T, w syncWorld) (string, string) {
	t.Helper()
	userID := givenMember(t, w, w.organizationID, w.managedID)
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
