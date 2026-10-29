package fga_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/fga/fgatest"
)

const docsModel = `model
  schema 1.1
type user
type team
  relations
    define member: [user]
type doc
  relations
    define owner: [user, team#member]
    define viewer: [user:*] or owner`

func ownerOf(doc, user string) fga.Tuple {
	return fga.Tuple{Object: fga.Ref("doc", doc), Relation: "owner", User: fga.Ref("user", user)}
}

func TestDiff(t *testing.T) {
	t.Run("Writes what is missing and deletes what is left over", func(t *testing.T) {
		keep, drop, add := ownerOf("a", "ana"), ownerOf("b", "ana"), ownerOf("c", "ana")

		changes := fga.Diff([]fga.Tuple{keep, drop, drop}, []fga.Tuple{keep, add, add})

		assert.Equal(t, []fga.Tuple{add}, changes.Writes)
		assert.Equal(t, []fga.Tuple{drop}, changes.Deletes)
	})

	t.Run("Equal sides need no change", func(t *testing.T) {
		same := []fga.Tuple{ownerOf("a", "ana"), ownerOf("b", "bob")}

		assert.True(t, fga.Diff(same, same).IsEmpty())
	})
}

func TestReferences(t *testing.T) {
	t.Run("A reference splits back into its type and ID", func(t *testing.T) {
		assert.Equal(t, "doc", fga.TypeOf("doc:readme"))
		assert.Equal(t, "readme", fga.IDOf("doc:readme"))
		assert.Equal(t, "core", fga.IDOf("team:core#member"))
		assert.Equal(t, "user:*", fga.Wildcard("user"))
	})
}

func TestStore(t *testing.T) {
	engine := fgatest.NewEngine(t)

	t.Run("Apply writes, then deletes, and a check follows the tuples", func(t *testing.T) {
		store := fgatest.StoreWithModel(t, engine, docsModel)
		ctx := t.Context()

		require.NoError(t, store.Apply(ctx, fga.Changes{Writes: []fga.Tuple{ownerOf("readme", "ana")}}))
		allowed, err := store.Check(ctx, "user:ana", "viewer", "doc:readme")
		require.NoError(t, err)
		assert.True(t, allowed)

		require.NoError(t, store.Apply(ctx, fga.Changes{Deletes: []fga.Tuple{ownerOf("readme", "ana")}}))
		allowed, err = store.Check(ctx, "user:ana", "viewer", "doc:readme")
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("A tuple written twice or deleted twice is not an error", func(t *testing.T) {
		store := fgatest.StoreWithModel(t, engine, docsModel)
		tuple := ownerOf("readme", "ana")

		require.NoError(t, store.Apply(t.Context(), fga.Changes{Writes: []fga.Tuple{tuple}}))
		require.NoError(t, store.Apply(t.Context(), fga.Changes{Writes: []fga.Tuple{tuple}}))
		require.NoError(t, store.Apply(t.Context(), fga.Changes{Deletes: []fga.Tuple{tuple}}))
		require.NoError(t, store.Apply(t.Context(), fga.Changes{Deletes: []fga.Tuple{tuple}}))
	})

	t.Run("More changes than one request holds are applied in batches", func(t *testing.T) {
		store := fgatest.StoreWithModel(t, engine, docsModel)
		var tuples []fga.Tuple
		for i := range 250 {
			tuples = append(tuples, ownerOf(fmt.Sprintf("doc%03d", i), "ana"))
		}

		require.NoError(t, store.Apply(t.Context(), fga.Changes{Writes: tuples}))

		stored, err := store.Read(t.Context(), fga.Filter{})
		require.NoError(t, err)
		assert.Len(t, stored, 250)
	})

	t.Run("Read follows every page and honours the filter", func(t *testing.T) {
		store := fgatest.StoreWithModel(t, engine, docsModel)
		var tuples []fga.Tuple
		for i := range 150 {
			tuples = append(tuples, ownerOf("readme", fmt.Sprintf("u%03d", i)))
		}
		tuples = append(tuples, ownerOf("other", "ana"))
		require.NoError(t, store.Apply(t.Context(), fga.Changes{Writes: tuples}))

		readme, err := store.Read(t.Context(), fga.Filter{Object: "doc:readme", Relation: "owner"})
		require.NoError(t, err)

		assert.Len(t, readme, 150)
	})

	t.Run("CheckAll answers more checks than one BatchCheck takes, in order", func(t *testing.T) {
		store := fgatest.StoreWithModel(t, engine, docsModel)
		require.NoError(t, store.Apply(t.Context(), fga.Changes{Writes: []fga.Tuple{ownerOf("readme", "ana")}}))
		var checks []fga.Tuple
		for i := range 55 {
			checks = append(
				checks,
				fga.Tuple{Object: fmt.Sprintf("doc:d%02d", i), Relation: "viewer", User: "user:ana"},
			)
		}
		checks = append(checks, fga.Tuple{Object: "doc:readme", Relation: "viewer", User: "user:ana"})

		answers, err := store.CheckAll(t.Context(), checks)

		require.NoError(t, err)
		require.Len(t, answers, 56)
		assert.NotContains(t, answers[:55], true)
		assert.True(t, answers[55])
	})

	t.Run("CheckAll fails on a relation the model doesn't define", func(t *testing.T) {
		store := fgatest.StoreWithModel(t, engine, docsModel)

		_, err := store.CheckAll(t.Context(), []fga.Tuple{{Object: "doc:readme", Relation: "editor", User: "user:ana"}})

		require.ErrorIs(t, err, fga.ErrCheckItem)
	})

	t.Run("Relations reads what the store's model defines", func(t *testing.T) {
		store := fgatest.StoreWithModel(t, engine, docsModel)

		relations, err := store.Relations(t.Context())

		require.NoError(t, err)
		assert.True(t, relations.Defines(ownerOf("readme", "ana")))
		assert.False(t, relations.Defines(fga.Tuple{Object: "doc:readme", Relation: "editor", User: "user:ana"}))
	})
}

func TestRelations(t *testing.T) {
	relations := fga.RelationsOf(fgatest.Model(t, docsModel))
	doc := func(relation, user string) fga.Tuple {
		return fga.Tuple{Object: "doc:readme", Relation: relation, User: user}
	}

	t.Run("A relation accepts the user types it takes directly", func(t *testing.T) {
		assert.True(t, relations.Accepts(doc("owner", "user:ana")))
		assert.True(t, relations.Accepts(doc("owner", "team:core#member")))
		assert.True(t, relations.Accepts(doc("viewer", "user:*")))
	})

	t.Run("A relation refuses a user type, wildcard or userset it doesn't take", func(t *testing.T) {
		assert.False(t, relations.Accepts(doc("owner", "team:core")))
		assert.False(t, relations.Accepts(doc("owner", "user:*")))
		assert.False(t, relations.Accepts(doc("viewer", "user:ana")))
		assert.False(t, relations.Accepts(doc("editor", "user:ana")))
	})

	t.Run("A relation is defined for checks of a user type it doesn't take", func(t *testing.T) {
		assert.True(t, relations.Defines(doc("viewer", "user:ana")))
		assert.False(t, relations.Defines(doc("editor", "user:ana")))
	})
}
