package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/google/uuid"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	accessservice "anchor/internal/access/service"
	"anchor/internal/applog"
	"anchor/internal/authz/decision"
	authzengine "anchor/internal/authz/engine"
	"anchor/internal/authz/fga"
	"anchor/internal/authz/fgasync"
	"anchor/internal/authz/model"
	authzsource "anchor/internal/authz/source"
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

	t.Run(
		"The first decision after a release with a new model version writes its own row without touching the previous release",
		func(t *testing.T) {
			w := givenSyncWorld(t)
			externalID, _ := givenMemberWithExternalID(t, w)
			rebuild(t, w.productID)

			// Move the existing model onto a previous release version so this release
			// arrives on an already-rebuilt store with no row for model.Version yet.
			var prevModelID string
			require.NoError(t, TestDB.QueryRowContext(t.Context(),
				`UPDATE authz_models SET model_version = 'previous-release', model_hash = 'previous-release-hash'
			 WHERE product_id = $1 AND model_version = $2 RETURNING model_id`,
				w.productID, model.Version).Scan(&prevModelID))
			_, err := TestDB.ExecContext(t.Context(),
				`UPDATE authz_stores SET model_hash = 'previous-release-hash' WHERE product_id = $1`,
				w.productID)
			require.NoError(t, err)

			checker := decision.NewChecker(AuthzStores, AuthzSyncer, applog.Scoped{Logger: TestLogger})
			evaluator := accessservice.NewEvaluator(ProductUserRepo, MembershipRepo, OrganizationRepo,
				OrgAPIKeyService, checker, ResourceTypeService, ResourceRepo, RoleAssignmentRepo, TestLogger,
				applog.Scoped{Logger: TestLogger})

			decided, err := evaluator.Evaluate(t.Context(), userRequest(w, externalID))

			require.NoError(t, err)
			assert.True(t, decided.Allowed)

			// Previous release keeps its untouched row, while this release wrote its own.
			var prevAfter string
			require.NoError(t, TestDB.QueryRowContext(t.Context(),
				`SELECT model_id FROM authz_models WHERE product_id = $1 AND model_version = 'previous-release'`,
				w.productID).Scan(&prevAfter))
			assert.Equal(t, prevModelID, prevAfter, "previous release's model row must stay untouched")

			current, err := AuthzMarks.RecordedModel(t.Context(), w.productID, model.Version)
			require.NoError(t, err)
			assert.NotEmpty(t, current.ID)
			assert.NotEqual(t, prevModelID, current.ID)
			assert.NotEqual(t, "previous-release-hash", current.Hash)
		},
	)

	t.Run("Alternating decisions across two release model versions write no extra models", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, _ := givenMemberWithExternalID(t, w)
		rebuild(t, w.productID)

		const nextVersion = "anchor-authz-model-next"
		nextStores := authzstore.NewWithVersion(
			TestDB, AuthzEngines, TestLogger, applog.Scoped{Logger: TestLogger}, nextVersion,
		)
		nextSyncer, err := fgasync.New(fgasync.Deps{
			Engines:    AuthzEngines,
			Stores:     nextStores,
			Repository: AuthzMarks,
			Locker:     fgasync.NewPostgresLocker(Transactor, nil),
			Models: versionedModels{
				base:    authzsource.NewCatalog(authzsource.NewTables(TestDB), ResourceTypeService),
				version: nextVersion,
			},
			Sources: nil,
			Logger:  TestLogger,
		})
		require.NoError(t, err)

		makeEval := func(s authzstore.Stores, r decision.Rebuilder) accessservice.Evaluator {
			c := decision.NewChecker(s, r, applog.Scoped{Logger: TestLogger})
			return accessservice.NewEvaluator(ProductUserRepo, MembershipRepo, OrganizationRepo,
				OrgAPIKeyService, c, ResourceTypeService, ResourceRepo, RoleAssignmentRepo, TestLogger,
				applog.Scoped{Logger: TestLogger})
		}

		// First decision on the next release creates its own authz_models row.
		decided, err := makeEval(nextStores, nextSyncer).Evaluate(t.Context(), userRequest(w, externalID))
		require.NoError(t, err)
		assert.True(t, decided.Allowed)

		storeID, err := AuthzStores.StoreID(t.Context(), w.productID)
		require.NoError(t, err)
		beforeResp, err := openFGA(
			t,
		).ReadAuthorizationModels(t.Context(), &openfgav1.ReadAuthorizationModelsRequest{StoreId: storeID})
		require.NoError(t, err)
		beforeCount := len(beforeResp.GetAuthorizationModels())

		// Alternating fresh checkers (forcing EnsureModel on every call) writes zero new models.
		for range 3 {
			d1, evalErr := makeEval(AuthzStores, AuthzSyncer).Evaluate(t.Context(), userRequest(w, externalID))
			require.NoError(t, evalErr)
			assert.True(t, d1.Allowed)
			d2, evalErr := makeEval(nextStores, nextSyncer).Evaluate(t.Context(), userRequest(w, externalID))
			require.NoError(t, evalErr)
			assert.True(t, d2.Allowed)
		}

		afterResp, err := openFGA(
			t,
		).ReadAuthorizationModels(t.Context(), &openfgav1.ReadAuthorizationModelsRequest{StoreId: storeID})
		require.NoError(t, err)
		assert.Len(
			t,
			afterResp.GetAuthorizationModels(),
			beforeCount,
			"alternating releases must not write new OpenFGA models",
		)
	})

	t.Run("An engine that isn't ready answers 503", func(t *testing.T) {
		w := givenSyncWorld(t)
		externalID, _ := givenMemberWithExternalID(t, w)
		checker := decision.NewChecker(
			authzstore.New(TestDB, notReady{}, TestLogger, applog.Scoped{Logger: TestLogger}), AuthzSyncer,
			applog.Scoped{Logger: TestLogger},
		)
		evaluator := accessservice.NewEvaluator(ProductUserRepo, MembershipRepo, OrganizationRepo,
			OrgAPIKeyService, checker, ResourceTypeService, ResourceRepo, RoleAssignmentRepo, TestLogger,
			applog.Scoped{Logger: TestLogger})

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

type versionedModels struct {
	base    fgasync.ModelSource
	version string
}

func (v versionedModels) Model(ctx context.Context, productID string) (fgasync.Model, error) {
	m, err := v.base.Model(ctx, productID)
	if err != nil {
		return fgasync.Model{}, err
	}
	m.Version = v.version
	sum := sha256.Sum256([]byte(v.version + "\n" + m.Hash))
	m.Hash = hex.EncodeToString(sum[:])
	return m, nil
}
