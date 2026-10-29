// Package fgatest runs a real OpenFGA server in memory for unit tests, so a
// test exercises OpenFGA's own read, write and check semantics without Postgres.
package fgatest

import (
	"testing"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/language/pkg/go/transformer"
	"github.com/openfga/openfga/pkg/server"
	"github.com/openfga/openfga/pkg/storage/memory"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/fga"
)

// NewEngine starts an OpenFGA server on an in-memory datastore, closed when
// the test ends.
func NewEngine(t testing.TB) fga.Engine {
	t.Helper()
	datastore := memory.New()
	openfga, err := server.NewServerWithOpts(
		server.WithDatastore(datastore),
		server.WithCheckQueryCacheEnabled(false),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		openfga.Close()
		datastore.Close()
	})
	return openfga
}

// NewStore creates a store on the engine and returns its ID.
func NewStore(t testing.TB, engine fga.Engine) string {
	t.Helper()
	created, err := engine.CreateStore(t.Context(), &openfgav1.CreateStoreRequest{Name: "test-store"})
	require.NoError(t, err)
	return created.GetId()
}

// Model compiles an authorization model written in OpenFGA's DSL.
func Model(t testing.TB, dsl string) *openfgav1.AuthorizationModel {
	t.Helper()
	model, err := transformer.TransformDSLToProto(dsl)
	require.NoError(t, err)
	return model
}

// StoreWithModel creates a store, writes the model and returns the store at it.
func StoreWithModel(t testing.TB, engine fga.Engine, dsl string) fga.Store {
	t.Helper()
	storeID := NewStore(t, engine)
	modelID, err := fga.WriteModel(t.Context(), engine, storeID, Model(t, dsl))
	require.NoError(t, err)
	return fga.NewStore(engine, storeID, modelID)
}
