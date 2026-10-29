package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/fga"
	"anchor/internal/authz/model"
	"anchor/internal/authz/store"
)

// openFGA waits for the engine, which starts beside the application.
func openFGA(t *testing.T) engine.Engine {
	t.Helper()
	var ready engine.Engine
	require.Eventually(t, func() bool {
		var err error
		ready, err = AuthzEngines.Get()
		return err == nil
	}, 30*time.Second, 50*time.Millisecond, "openfga never became ready")
	return ready
}

func TestAuthzStores(t *testing.T) {
	openfga := openFGA(t)

	t.Run("A product gets one store, and asking again returns it", func(t *testing.T) {
		productID := GivenATenantAndProduct(t).Product.ID

		first, err := AuthzStores.StoreID(t.Context(), productID)
		require.NoError(t, err)
		again, err := AuthzStores.StoreID(t.Context(), productID)
		require.NoError(t, err)

		assert.NotEmpty(t, first)
		assert.Equal(t, first, again)
	})

	t.Run(
		"Open returns the store at its model once rebuilt, and Holds skips an undefined permission",
		func(t *testing.T) {
			w := givenSyncWorld(t)
			userID := givenMember(t, w, w.organizationID, w.managedID)
			rebuild(t, w.productID)

			found, err := AuthzStores.Open(t.Context(), w.productID)

			require.NoError(t, err)
			opened, ok := found.Get()
			require.True(t, ok)
			assert.Equal(t, mustStoreID(t, w.productID), opened.Store.ID())
			assert.True(t, opened.Rebuilt)
			held, err := opened.Holds(t.Context(), []fga.Tuple{
				{
					Object:   model.Organization(w.organizationID),
					Relation: model.CanRelation(w.permission),
					User:     model.User(userID),
				},
				{
					Object:   model.Organization(w.organizationID),
					Relation: model.CanRelation("undeclared:x"),
					User:     model.User(userID),
				},
			})
			require.NoError(t, err)
			assert.Equal(t, []bool{true, false}, held)
		},
	)

	t.Run("Open finds nothing for a product without a store, and creates none", func(t *testing.T) {
		productID := "product_" + uuid.NewString()

		opened, err := AuthzStores.Open(t.Context(), productID)

		require.NoError(t, err)
		assert.True(t, opened.IsAbsent())
		var stores int
		require.NoError(t, TestDB.QueryRowContext(t.Context(),
			`SELECT count(*) FROM authz_stores WHERE product_id = $1`, productID).Scan(&stores))
		assert.Zero(t, stores)
	})

	t.Run("Replicas asking at once keep one store", func(t *testing.T) {
		productID := GivenATenantAndProduct(t).Product.ID
		const askers = 10
		ids := make([]string, askers)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range askers {
			wg.Go(func() {
				<-start
				id, err := AuthzStores.StoreID(t.Context(), productID)
				assert.NoError(t, err)
				ids[i] = id
			})
		}
		close(start)
		wg.Wait()

		for _, id := range ids {
			assert.Equal(t, ids[0], id)
		}
		var recorded int
		require.NoError(t, TestDB.QueryRowContext(t.Context(),
			`SELECT count(*) FROM authz_stores WHERE product_id = $1`, productID).Scan(&recorded))
		assert.Equal(t, 1, recorded)
		live, err := openfga.ListStores(t.Context(), &openfgav1.ListStoresRequest{Name: productID})
		require.NoError(t, err)
		assert.Len(t, live.GetStores(), 1, "the stores that lost the race are deleted")
	})

	t.Run("Asking inside a transaction is refused", func(t *testing.T) {
		productID := GivenATenantAndProduct(t).Product.ID

		err := Transactor.InTx(t.Context(), func(txCtx context.Context) error {
			_, storeErr := AuthzStores.StoreID(txCtx, productID)
			return storeErr
		})

		assert.ErrorIs(t, err, store.ErrInTransaction)
	})

	t.Run("A product that does not exist gets no store", func(t *testing.T) {
		_, err := AuthzStores.StoreID(t.Context(), "prd_missing")

		assert.ErrorIs(t, err, store.ErrProductNotFound)
	})

	t.Run("The engine answers a check from the store's model and tuples", func(t *testing.T) {
		ctx := t.Context()
		storeID, err := AuthzStores.StoreID(ctx, GivenATenantAndProduct(t).Product.ID)
		require.NoError(t, err)
		model, err := openfga.WriteAuthorizationModel(ctx, &openfgav1.WriteAuthorizationModelRequest{
			StoreId:       storeID,
			SchemaVersion: "1.1",
			TypeDefinitions: []*openfgav1.TypeDefinition{
				{Type: "user"},
				{
					Type: "organization",
					Relations: map[string]*openfgav1.Userset{
						"member": {Userset: &openfgav1.Userset_This{This: &openfgav1.DirectUserset{}}},
					},
					Metadata: &openfgav1.Metadata{Relations: map[string]*openfgav1.RelationMetadata{
						"member": {DirectlyRelatedUserTypes: []*openfgav1.RelationReference{{Type: "user"}}},
					}},
				},
			},
		})
		require.NoError(t, err)
		_, err = openfga.Write(ctx, &openfgav1.WriteRequest{
			StoreId:              storeID,
			AuthorizationModelId: model.GetAuthorizationModelId(),
			Writes: &openfgav1.WriteRequestWrites{TupleKeys: []*openfgav1.TupleKey{
				{User: "user:alice", Relation: "member", Object: "organization:acme"},
			}},
		})
		require.NoError(t, err)

		allowed, err := openfga.Check(ctx, &openfgav1.CheckRequest{
			StoreId:              storeID,
			AuthorizationModelId: model.GetAuthorizationModelId(),
			TupleKey: &openfgav1.CheckRequestTupleKey{
				User:     "user:alice",
				Relation: "member",
				Object:   "organization:acme",
			},
		})
		require.NoError(t, err)
		denied, err := openfga.Check(ctx, &openfgav1.CheckRequest{
			StoreId:              storeID,
			AuthorizationModelId: model.GetAuthorizationModelId(),
			TupleKey: &openfgav1.CheckRequestTupleKey{
				User:     "user:bob",
				Relation: "member",
				Object:   "organization:acme",
			},
		})
		require.NoError(t, err)

		assert.True(t, allowed.GetAllowed())
		assert.False(t, denied.GetAllowed())
	})
}

func mustStoreID(t *testing.T, productID string) string {
	t.Helper()
	id, err := AuthzStores.StoreID(t.Context(), productID)
	require.NoError(t, err)
	return id
}
