package fgasync_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/language/pkg/go/transformer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/fga/fgatest"
	"anchor/internal/authz/fgasync"
	"anchor/internal/authz/fgasync/fgasynctest"
	"anchor/internal/events"
)

// The tests sync a toy domain: documents and their owners, in product
// prod_docs. docOwners is its source, keyed by document.

const (
	productID     = "prod_docs"
	docChanged    = events.Type("doc.changed")
	modelChanged  = events.Type("model.changed")
	fieldDocID    = "doc_id"
	ownersModel   = "model\n  schema 1.1\ntype user\ntype doc\n  relations\n    define owner: [user]\n"
	ownersEditors = ownersModel + "    define editor: [user]\n"
)

// docs is the source of truth: each document's owners, per Product.
type docs struct {
	mu     sync.Mutex
	owners map[string]map[string][]string
	// during runs inside Desired, to land a write in the middle of a sync.
	during func()
	failOn string
}

func (d *docs) set(productID, doc string, owners ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.owners[productID] == nil {
		d.owners[productID] = map[string][]string{}
	}
	if owners == nil {
		delete(d.owners[productID], doc)
		return
	}
	d.owners[productID][doc] = owners
}

type docOwners struct{ docs *docs }

func (docOwners) Name() string { return "doc_owners" }

func (docOwners) Stale(event events.Event) ([]string, error) {
	if event.Type != docChanged {
		return nil, nil
	}
	return []string{event.Data[fieldDocID]}, nil
}

func (p docOwners) Keys(_ context.Context, productID string) ([]string, error) {
	p.docs.mu.Lock()
	defer p.docs.mu.Unlock()
	var keys []string
	for doc := range p.docs.owners[productID] {
		keys = append(keys, doc)
	}
	slices.Sort(keys)
	return keys, nil
}

func (p docOwners) Desired(_ context.Context, productID, doc string) ([]fga.Tuple, error) {
	if p.docs.during != nil {
		during := p.docs.during
		p.docs.during = nil
		during()
	}
	if productID == p.docs.failOn {
		return nil, errors.New("tables unreachable")
	}
	p.docs.mu.Lock()
	defer p.docs.mu.Unlock()
	var tuples []fga.Tuple
	for _, owner := range p.docs.owners[productID][doc] {
		tuples = append(tuples, owns(doc, owner))
	}
	return tuples, nil
}

func (docOwners) Stored(ctx context.Context, store fga.Reader, doc string) ([]fga.Tuple, error) {
	return store.Read(ctx, fga.Filter{Object: fga.Ref("doc", doc), Relation: "owner"})
}

func owns(doc, user string) fga.Tuple {
	return fga.Tuple{Object: fga.Ref("doc", doc), Relation: "owner", User: fga.Ref("user", user)}
}

// models serves one DSL for every Product and counts compilations.
type models struct {
	mu       sync.Mutex
	dsl      string
	compiles int
}

func (m *models) Model(_ context.Context, _ string) (fgasync.Model, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dsl := m.dsl
	return fgasync.Model{Hash: dsl, Compile: func() (*openfgav1.AuthorizationModel, error) {
		m.mu.Lock()
		m.compiles++
		m.mu.Unlock()
		return transformer.TransformDSLToProto(dsl)
	}}, nil
}

type world struct {
	syncer *fgasync.Syncer
	marker *fgasync.Marker
	docs   *docs
	models *models
	repo   *fgasynctest.Repository
	locker *fgasynctest.Locker
	stores *fgasynctest.Stores
	engine fga.Engine
}

func givenWorld(t *testing.T) world {
	t.Helper()
	engine := fgatest.NewEngine(t)
	w := world{
		docs:   &docs{owners: map[string]map[string][]string{}},
		models: &models{dsl: ownersModel},
		repo:   fgasynctest.NewRepository(time.Now),
		locker: fgasynctest.NewLocker(),
		stores: fgasynctest.NewStores(engine),
		engine: engine,
	}
	w.repo.AddProduct(productID)
	m, err := fgasync.New(fgasync.Deps{
		Engines: fgasynctest.Engines{Engine: engine}, Stores: w.stores, Repository: w.repo, Locker: w.locker,
		Models: w.models, Sources: []fgasync.Source{docOwners{docs: w.docs}}, Logger: zerolog.Nop(),
	})
	require.NoError(t, err)
	w.syncer = m
	w.marker, err = fgasync.NewMarker(
		w.repo,
		[]fgasync.Source{docOwners{docs: w.docs}},
		func(event events.Event) bool {
			return event.Type == modelChanged
		},
	)
	require.NoError(t, err)
	return w
}

func (w world) changeDoc(ctx context.Context, t *testing.T, productID, doc string, owners ...string) {
	t.Helper()
	w.docs.set(productID, doc, owners...)
	require.NoError(t, w.marker.ObserveTx(ctx, events.Event{
		Type: docChanged, ProductID: productID, Data: events.Data{fieldDocID: doc},
	}))
}

func (w world) stored(t *testing.T, productID string) []fga.Tuple {
	t.Helper()
	tuples, err := w.stores.Store(t, w.repo, productID).Read(t.Context(), fga.Filter{})
	require.NoError(t, err)
	fga.Sort(tuples)
	return tuples
}

func readmeScope() fgasync.Scope {
	return fgasync.Scope{ProductID: productID, Source: "doc_owners", Key: "readme"}
}

func TestSync(t *testing.T) {
	t.Run("A key's sync writes what the key implies and clears its mark", func(t *testing.T) {
		w := givenWorld(t)
		w.changeDoc(t.Context(), t, productID, "readme", "ana", "bob")

		require.NoError(t, w.syncer.Sync(t.Context(), readmeScope()))

		assert.Equal(t, []fga.Tuple{owns("readme", "ana"), owns("readme", "bob")}, w.stored(t, productID))
		assert.Empty(t, w.repo.Marks())
	})

	t.Run("A key's sync deletes what the key no longer implies", func(t *testing.T) {
		w := givenWorld(t)
		w.changeDoc(t.Context(), t, productID, "readme", "ana", "bob")
		require.NoError(t, w.syncer.Sync(t.Context(), readmeScope()))
		w.changeDoc(t.Context(), t, productID, "readme", "bob")

		require.NoError(t, w.syncer.Sync(t.Context(), readmeScope()))

		assert.Equal(t, []fga.Tuple{owns("readme", "bob")}, w.stored(t, productID))
	})

	t.Run("A key's sync leaves the tuples of other keys alone", func(t *testing.T) {
		w := givenWorld(t)
		w.changeDoc(t.Context(), t, productID, "readme", "ana")
		w.changeDoc(t.Context(), t, productID, "notes", "bob")
		require.NoError(t, w.syncer.Sync(t.Context(), fgasync.WholeProduct(productID)))
		w.changeDoc(t.Context(), t, productID, "readme")

		require.NoError(t, w.syncer.Sync(t.Context(), readmeScope()))

		assert.Equal(t, []fga.Tuple{owns("notes", "bob")}, w.stored(t, productID))
	})

	t.Run("A write landing during a sync keeps its mark for the next sync", func(t *testing.T) {
		w := givenWorld(t)
		w.changeDoc(t.Context(), t, productID, "readme", "ana")
		w.docs.during = func() { w.changeDoc(t.Context(), t, productID, "readme", "ana", "carol") }

		require.NoError(t, w.syncer.Sync(t.Context(), readmeScope()))

		assert.Equal(t, []fgasync.Scope{readmeScope()}, w.repo.Marks())
		require.NoError(t, w.syncer.Sync(t.Context(), readmeScope()))
		assert.Equal(t, []fga.Tuple{owns("readme", "ana"), owns("readme", "carol")}, w.stored(t, productID))
		assert.Empty(t, w.repo.Marks())
	})

	t.Run("Every key of a source includes a key only its mark still names", func(t *testing.T) {
		w := givenWorld(t)
		w.changeDoc(t.Context(), t, productID, "readme", "ana")
		w.changeDoc(t.Context(), t, productID, "notes", "bob")
		require.NoError(t, w.syncer.Sync(t.Context(), fgasync.WholeProduct(productID)))
		w.changeDoc(t.Context(), t, productID, "notes")

		require.NoError(t, w.syncer.Sync(t.Context(), fgasync.Scope{
			ProductID: productID, Source: "doc_owners", Key: fgasync.EveryKey,
		}))

		assert.Equal(t, []fga.Tuple{owns("readme", "ana")}, w.stored(t, productID))
		assert.Empty(t, w.repo.Marks())
	})

	t.Run("A mark of a source that no longer exists syncs the whole Product and goes", func(t *testing.T) {
		w := givenWorld(t)
		w.docs.set(productID, "readme", "ana")
		gone := fgasync.Scope{ProductID: productID, Source: "renamed_away", Key: "x"}
		require.NoError(t, w.repo.Mark(t.Context(), gone))

		require.NoError(t, w.syncer.Sync(t.Context(), gone))

		assert.Equal(t, []fga.Tuple{owns("readme", "ana")}, w.stored(t, productID))
		assert.Empty(t, w.repo.Marks())
	})

	t.Run("A sync waits for a busy lock, then fails and keeps its mark", func(t *testing.T) {
		w := givenWorld(t)
		w.syncer.ShortenLockWait(20 * time.Millisecond)
		w.changeDoc(t.Context(), t, productID, "readme", "ana")
		require.True(t, w.locker.Hold(productID))

		err := w.syncer.Sync(t.Context(), readmeScope())

		require.ErrorIs(t, err, fgasync.ErrLockBusy)
		assert.Equal(t, []fgasync.Scope{readmeScope()}, w.repo.Marks())
	})

	t.Run("Another key syncs while one key is busy, and the busy key waits", func(t *testing.T) {
		w := givenWorld(t)
		w.syncer.ShortenLockWait(20 * time.Millisecond)
		w.changeDoc(t.Context(), t, productID, "readme", "ana")
		w.changeDoc(t.Context(), t, productID, "notes", "bob")
		require.NoError(t, w.syncer.Sync(t.Context(), fgasync.WholeProduct(productID)))
		w.changeDoc(t.Context(), t, productID, "readme", "carol")
		w.changeDoc(t.Context(), t, productID, "notes", "dan")
		require.True(t, w.locker.HoldKey(productID, "doc_owners:readme"))
		defer w.locker.ReleaseKey(productID, "doc_owners:readme")

		notesErr := w.syncer.Sync(t.Context(), fgasync.Scope{ProductID: productID, Source: "doc_owners", Key: "notes"})
		readmeErr := w.syncer.Sync(t.Context(), readmeScope())

		require.NoError(t, notesErr)
		require.ErrorIs(t, readmeErr, fgasync.ErrLockBusy)
		assert.Equal(t, []fga.Tuple{owns("notes", "dan"), owns("readme", "ana")}, w.stored(t, productID))
		assert.Equal(t, []fgasync.Scope{readmeScope()}, w.repo.Marks())
	})
}

func TestModel(t *testing.T) {
	t.Run("A model change marks the whole Product, and its sync writes the new model", func(t *testing.T) {
		w := givenWorld(t)
		w.changeDoc(t.Context(), t, productID, "readme", "ana")
		require.NoError(t, w.syncer.Sync(t.Context(), readmeScope()))
		before, err := w.repo.RecordedModel(t.Context(), productID)
		require.NoError(t, err)
		w.models.dsl = ownersEditors

		require.NoError(t, w.marker.ObserveTx(t.Context(), events.Event{Type: modelChanged, ProductID: productID}))

		assert.Equal(t, []fgasync.Scope{fgasync.WholeProduct(productID)}, w.repo.Marks())
		require.NoError(t, w.syncer.Sync(t.Context(), fgasync.WholeProduct(productID)))
		after, err := w.repo.RecordedModel(t.Context(), productID)
		require.NoError(t, err)
		assert.NotEqual(t, before.ID, after.ID)
		assert.Equal(t, ownersEditors, after.Hash)
	})

	t.Run("An unchanged model is compiled once and never written again", func(t *testing.T) {
		w := givenWorld(t)
		w.changeDoc(t.Context(), t, productID, "readme", "ana")
		require.NoError(t, w.syncer.Sync(t.Context(), readmeScope()))
		first, err := w.repo.RecordedModel(t.Context(), productID)
		require.NoError(t, err)

		for range 3 {
			require.NoError(t, w.syncer.Sync(t.Context(), readmeScope()))
		}

		again, err := w.repo.RecordedModel(t.Context(), productID)
		require.NoError(t, err)
		assert.Equal(t, first.ID, again.ID)
		assert.Equal(t, 1, w.models.compiles)
	})

	t.Run("A tuple whose relation the model lacks is dropped, not written", func(t *testing.T) {
		w := givenWorld(t)
		w.models.dsl = ownersEditors
		w.changeDoc(t.Context(), t, productID, "readme", "ana")
		require.NoError(t, w.syncer.Sync(t.Context(), fgasync.WholeProduct(productID)))
		w.models.dsl = ownersModel
		w.docs.set(productID, "readme", "ana")
		require.NoError(t, w.marker.ObserveTx(t.Context(), events.Event{Type: modelChanged, ProductID: productID}))

		require.NoError(t, w.syncer.Sync(t.Context(), fgasync.WholeProduct(productID)))

		assert.Equal(t, []fga.Tuple{owns("readme", "ana")}, w.stored(t, productID))
	})
}

func TestMarks(t *testing.T) {
	t.Run("An event no source claims marks nothing", func(t *testing.T) {
		w := givenWorld(t)

		require.NoError(t, w.marker.ObserveTx(t.Context(), events.Event{Type: "invoice.paid", ProductID: productID}))

		assert.Empty(t, w.repo.Marks())
	})

	t.Run("A source that marks an empty key fails the write", func(t *testing.T) {
		w := givenWorld(t)

		err := w.marker.ObserveTx(
			t.Context(),
			events.Event{Type: docChanged, ProductID: productID, Data: events.Data{}},
		)

		require.ErrorIs(t, err, fgasync.ErrInvalidScope)
		assert.Empty(t, w.repo.Marks())
	})

	t.Run("A request's marks are collected, and its replay syncs them", func(t *testing.T) {
		w := givenWorld(t)
		ctx, marked := fgasync.CollectMarks(t.Context())
		w.changeDoc(ctx, t, productID, "readme", "ana")
		w.changeDoc(ctx, t, productID, "readme", "ana", "bob")

		w.syncer.Replay(t.Context(), marked())

		assert.Equal(t, []fgasync.Scope{readmeScope()}, marked())
		assert.Equal(t, []fga.Tuple{owns("readme", "ana"), owns("readme", "bob")}, w.stored(t, productID))
		assert.Empty(t, w.repo.Marks())
	})

	t.Run("A replay that cannot get the lock leaves the mark for the sweeper", func(t *testing.T) {
		w := givenWorld(t)
		w.syncer.ShortenLockWait(20 * time.Millisecond)
		ctx, marked := fgasync.CollectMarks(t.Context())
		w.changeDoc(ctx, t, productID, "readme", "ana")
		require.True(t, w.locker.Hold(productID))

		w.syncer.Replay(t.Context(), marked())

		w.locker.Release(productID)
		assert.Equal(t, []fgasync.Scope{readmeScope()}, w.repo.Marks())
		w.syncer.Sweep(t.Context())
		assert.Equal(t, []fga.Tuple{owns("readme", "ana")}, w.stored(t, productID))
		assert.Empty(t, w.repo.Marks())
	})
}

func TestSweep(t *testing.T) {
	t.Run("A Product whose sync fails waits behind the others", func(t *testing.T) {
		w := givenWorld(t)
		w.repo.AddProduct("prod_other")
		w.changeDoc(t.Context(), t, productID, "readme", "ana")
		w.changeDoc(t.Context(), t, "prod_other", "notes", "bob")
		w.docs.failOn = productID

		w.syncer.Sweep(t.Context())

		assert.Equal(t, []fgasync.Scope{readmeScope()}, w.repo.Marks())
		assert.Equal(t, []fga.Tuple{owns("notes", "bob")}, w.stored(t, "prod_other"))
		w.changeDoc(t.Context(), t, "prod_other", "notes", "carol")
		oldest, err := w.repo.Oldest(t.Context(), 10)
		require.NoError(t, err)
		require.Len(t, oldest, 2)
		assert.Equal(t, "prod_other", oldest[0].ProductID, "the failed product's mark comes last")
	})

	t.Run("A Product never rebuilt is reconciled by the next sweep", func(t *testing.T) {
		w := givenWorld(t)
		w.docs.set(productID, "readme", "ana")

		w.syncer.Sweep(t.Context())

		assert.Equal(t, []fga.Tuple{owns("readme", "ana")}, w.stored(t, productID))
	})

	t.Run("A reconcile deletes every tuple no source claims", func(t *testing.T) {
		w := givenWorld(t)
		w.docs.set(productID, "readme", "ana")
		require.NoError(t, w.syncer.Reconcile(t.Context(), productID))
		stray := owns("ghost", "eve")
		require.NoError(t, w.stores.Store(t, w.repo, productID).Apply(t.Context(), fga.Changes{
			Writes: []fga.Tuple{stray},
		}))

		require.NoError(t, w.syncer.Reconcile(t.Context(), productID))

		assert.Equal(t, []fga.Tuple{owns("readme", "ana")}, w.stored(t, productID))
	})

	t.Run("A store whose Product is gone is deleted after a reconcile", func(t *testing.T) {
		w := givenWorld(t)
		w.repo.AddProduct("prod_gone")
		w.docs.set("prod_gone", "readme", "ana")
		require.NoError(t, w.syncer.Reconcile(t.Context(), "prod_gone"))
		goneStore, err := w.stores.StoreID(t.Context(), "prod_gone")
		require.NoError(t, err)
		w.repo.RemoveProduct("prod_gone")

		w.syncer.Sweep(t.Context())

		_, err = w.engine.GetStore(t.Context(), &openfgav1.GetStoreRequest{StoreId: goneStore})
		assert.Error(t, err)
	})
}

func TestNew(t *testing.T) {
	t.Run("Two sources with one name are refused", func(t *testing.T) {
		source := docOwners{docs: &docs{}}

		_, err := fgasync.New(fgasync.Deps{Sources: []fgasync.Source{source, source}})

		require.Error(t, err)
	})
}

func TestSmallestCover(t *testing.T) {
	key := func(product, source, key string) fgasync.Scope {
		return fgasync.Scope{ProductID: product, Source: source, Key: key}
	}

	t.Run("A whole Product covers its sources, and every key covers each key", func(t *testing.T) {
		cover := fgasync.SmallestCover([]fgasync.Scope{
			key("p1", "docs", "a"),
			key("p1", "docs", "a"),
			key("p2", "docs", "a"),
			key("p2", "docs", fgasync.EveryKey),
			key("p2", "teams", "t"),
			key("p1", "teams", "t"),
			fgasync.WholeProduct("p1"),
		})

		assert.Equal(t, []fgasync.Scope{
			key("p2", "docs", fgasync.EveryKey),
			key("p2", "teams", "t"),
			fgasync.WholeProduct("p1"),
		}, cover)
	})
}
