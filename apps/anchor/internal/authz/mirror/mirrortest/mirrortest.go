// Package mirrortest has in-memory adapters for the mirror's seams, so a
// projection or the mirror itself is tested against a real in-memory OpenFGA
// (fgatest) without Postgres.
package mirrortest

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/stretchr/testify/require"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/mirror"
)

// Repository keeps marks and store records in memory. Postgres runs a sync's
// reads and writes in the lock's transaction; this one applies them at once,
// which is what the mirror's correctness rests on anyway.
type Repository struct {
	mu         sync.Mutex
	revision   int64
	clock      func() time.Time
	marks      map[mirror.Scope]markRow
	models     map[string]mirror.RecordedModel
	reconciled map[string]time.Time
	products   map[string]bool
}

type markRow struct {
	revision int64
	markedAt time.Time
}

func NewRepository(clock func() time.Time) *Repository {
	return &Repository{
		clock:      clock,
		marks:      map[mirror.Scope]markRow{},
		models:     map[string]mirror.RecordedModel{},
		reconciled: map[string]time.Time{},
		products:   map[string]bool{},
	}
}

// AddProduct makes the Product exist, for ProductExists and DueForReconcile.
func (r *Repository) AddProduct(productID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.products[productID] = true
}

func (r *Repository) RemoveProduct(productID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.products, productID)
}

func (r *Repository) Mark(_ context.Context, scope mirror.Scope) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revision++
	r.marks[scope] = markRow{revision: r.revision, markedAt: r.clock()}
	return nil
}

func (r *Repository) Pending(_ context.Context, productID string) ([]mirror.Mark, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var pending []mirror.Mark
	for scope, row := range r.marks {
		if scope.ProductID == productID {
			pending = append(pending, mirror.Mark{Scope: scope, Revision: row.revision})
		}
	}
	slices.SortFunc(pending, func(a, b mirror.Mark) int { return int(a.Revision - b.Revision) })
	return pending, nil
}

func (r *Repository) ClearIfUnchanged(_ context.Context, mark mirror.Mark) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if row, found := r.marks[mark.Scope]; found && row.revision == mark.Revision {
		delete(r.marks, mark.Scope)
	}
	return nil
}

func (r *Repository) Oldest(_ context.Context, limit int) ([]mirror.Mark, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	type aged struct {
		mark mirror.Mark
		at   time.Time
	}
	var all []aged
	for scope, row := range r.marks {
		all = append(all, aged{mark: mirror.Mark{Scope: scope, Revision: row.revision}, at: row.markedAt})
	}
	slices.SortFunc(all, func(a, b aged) int {
		if c := a.at.Compare(b.at); c != 0 {
			return c
		}
		return int(a.mark.Revision - b.mark.Revision)
	})
	all = all[:min(limit, len(all))]
	return functional.Slice(all).Map(func(a aged) mirror.Mark { return a.mark }), nil
}

func (r *Repository) Postpone(_ context.Context, productID string, until time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for scope, row := range r.marks {
		if scope.ProductID == productID {
			r.marks[scope] = markRow{revision: row.revision, markedAt: until}
		}
	}
	return nil
}

func (r *Repository) RecordedModel(_ context.Context, productID string) (mirror.RecordedModel, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.models[productID], nil
}

func (r *Repository) SetModel(_ context.Context, productID string, model mirror.RecordedModel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[productID] = model
	return nil
}

func (r *Repository) SetReconciled(_ context.Context, productID string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reconciled[productID] = at
	return nil
}

func (r *Repository) DueForReconcile(_ context.Context, before time.Time) (functional.Option[string], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var due []string
	for productID := range r.products {
		if at, done := r.reconciled[productID]; !done || at.Before(before) {
			due = append(due, productID)
		}
	}
	if len(due) == 0 {
		return functional.None[string](), nil
	}
	slices.SortFunc(due, func(a, b string) int {
		return r.reconciled[a].Compare(r.reconciled[b])
	})
	return functional.Some(due[0]), nil
}

func (r *Repository) ProductExists(_ context.Context, productID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.products[productID], nil
}

// Marks lists the scopes that wait, for assertions.
func (r *Repository) Marks() []mirror.Scope {
	r.mu.Lock()
	defer r.mu.Unlock()
	scopes := make([]mirror.Scope, 0, len(r.marks))
	for scope := range r.marks {
		scopes = append(scopes, scope)
	}
	return scopes
}

// Locker is a per-Product mutex. Hold makes a Product's lock busy from the test.
type Locker struct {
	mu    sync.Mutex
	held  map[string]bool
	sweep sync.Mutex
}

func NewLocker() *Locker { return &Locker{held: map[string]bool{}} }

func (l *Locker) TryLocked(ctx context.Context, productID string, fn func(context.Context) error) (bool, error) {
	if !l.acquire(productID) {
		return false, nil
	}
	defer l.Release(productID)
	return true, fn(ctx)
}

func (l *Locker) TrySweep(ctx context.Context, fn func(context.Context) error) (bool, error) {
	if !l.sweep.TryLock() {
		return false, nil
	}
	defer l.sweep.Unlock()
	return true, fn(ctx)
}

// Hold takes the Product's lock as another sync would; Release frees it.
func (l *Locker) Hold(productID string) bool { return l.acquire(productID) }

func (l *Locker) Release(productID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.held, productID)
}

func (l *Locker) acquire(productID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held[productID] {
		return false
	}
	l.held[productID] = true
	return true
}

// Stores creates one store per Product on the engine, named after it.
type Stores struct {
	engine fga.Engine
	mu     sync.Mutex
	ids    map[string]string
}

func NewStores(engine fga.Engine) *Stores {
	return &Stores{engine: engine, ids: map[string]string{}}
}

func (s *Stores) StoreID(ctx context.Context, productID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, found := s.ids[productID]; found {
		return id, nil
	}
	created, err := s.engine.CreateStore(ctx, &openfgav1.CreateStoreRequest{Name: productID})
	if err != nil {
		return "", err
	}
	s.ids[productID] = created.GetId()
	return created.GetId(), nil
}

// Store returns the Product's store at the model the mirror recorded.
func (s *Stores) Store(t testing.TB, repo *Repository, productID string) fga.Store {
	t.Helper()
	storeID, err := s.StoreID(t.Context(), productID)
	require.NoError(t, err)
	recorded, err := repo.RecordedModel(t.Context(), productID)
	require.NoError(t, err)
	return fga.NewStore(s.engine, storeID, recorded.ID)
}

// Engines always hands out the engine.
type Engines struct{ Engine fga.Engine }

func (e Engines) Get() (fga.Engine, error) { return e.Engine, nil }
