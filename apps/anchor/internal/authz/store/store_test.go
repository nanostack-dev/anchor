package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/fga/fgatest"
	"anchor/internal/authz/store"
)

// A folder sits under the Organization and inherits what its members hold
// there; nothing else may be a folder's parent.
const folderModel = `model
  schema 1.1
type user
type organization
  relations
    define can_view: [user]
type rt_project
type rt_folder
  relations
    define parent: [organization]
    define can_view: can_view from parent`

const unregisteredFolder = "rt_folder:unregistered"

func canView(user string) fga.Tuple {
	return fga.Tuple{Object: unregisteredFolder, Relation: "can_view", User: user}
}

func parentOf(parent string) fga.Tuple {
	return fga.Tuple{Object: unregisteredFolder, Relation: "parent", User: parent}
}

func TestOpenedHolds(t *testing.T) {
	engine := fgatest.NewEngine(t)
	opened := func(t *testing.T) store.Opened {
		t.Helper()
		s := fgatest.StoreWithModel(t, engine, folderModel)
		relations, err := s.Relations(t.Context())
		require.NoError(t, err)
		require.NoError(t, s.Apply(t.Context(), fga.Changes{Writes: []fga.Tuple{
			{Object: "organization:acme", Relation: "can_view", User: "user:ana"},
		}}))
		return store.Opened{Store: s, Relations: relations, Rebuilt: true}
	}

	t.Run("A parent the model accepts holds for the check only", func(t *testing.T) {
		o := opened(t)

		held, err := o.Holds(t.Context(), []fga.Tuple{canView("user:ana"), canView("user:dan")},
			parentOf("organization:acme"))

		require.NoError(t, err)
		assert.Equal(t, []bool{true, false}, held)
	})

	t.Run("A parent the model refuses answers not held instead of an OpenFGA error", func(t *testing.T) {
		o := opened(t)

		held, err := o.Holds(t.Context(), []fga.Tuple{canView("user:ana")}, parentOf("rt_project:website"))

		require.NoError(t, err)
		assert.Equal(t, []bool{false}, held)
	})

	t.Run("A relation the model doesn't define is not held, and the others are still asked", func(t *testing.T) {
		o := opened(t)
		undefined := fga.Tuple{Object: unregisteredFolder, Relation: "can_delete", User: "user:ana"}

		held, err := o.Holds(t.Context(), []fga.Tuple{undefined, canView("user:ana")}, parentOf("organization:acme"))

		require.NoError(t, err)
		assert.Equal(t, []bool{false, true}, held)
	})
}
