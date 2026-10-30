package model_test

import (
	"testing"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/openfga/pkg/server"
	"github.com/openfga/openfga/pkg/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/model"
)

// writeWithLimit writes the model to an OpenFGA server that stores models of
// at most limit bytes.
func writeWithLimit(t *testing.T, compiled model.Model, limit int) error {
	t.Helper()
	datastore := memory.New()
	openfga, err := server.NewServerWithOpts(
		server.WithDatastore(datastore),
		server.WithMaxAuthorizationModelSizeInBytes(limit),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		openfga.Close()
		datastore.Close()
	})
	store, err := openfga.CreateStore(t.Context(), &openfgav1.CreateStoreRequest{Name: "size"})
	require.NoError(t, err)
	_, err = openfga.WriteAuthorizationModel(t.Context(), &openfgav1.WriteAuthorizationModelRequest{
		StoreId:         store.GetId(),
		SchemaVersion:   compiled.Proto.GetSchemaVersion(),
		TypeDefinitions: compiled.Proto.GetTypeDefinitions(),
		Conditions:      compiled.Proto.GetConditions(),
	})
	return err
}

func TestBytes(t *testing.T) {
	t.Run("OpenFGA stores a model at a limit of exactly its size and refuses it one byte below", func(t *testing.T) {
		compiled, err := model.Compile(documentCatalog())
		require.NoError(t, err)

		require.NoError(t, writeWithLimit(t, compiled, compiled.Bytes()))
		err = writeWithLimit(t, compiled, compiled.Bytes()-1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model exceeds size limit")
	})
}
