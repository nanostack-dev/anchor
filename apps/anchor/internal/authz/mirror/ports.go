package mirror

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/authz/fga"
)

// Repository keeps the mirror's bookkeeping: the marks, and the model and last
// rebuild of each Product's store. NewPostgresRepository is the production
// adapter; mirrortest has an in-memory one.
type Repository interface {
	// Mark records the scope as stale in the caller's transaction, giving it a
	// new revision.
	Mark(ctx context.Context, scope Scope) error
	// Pending lists the Product's marks.
	Pending(ctx context.Context, productID string) ([]Mark, error)
	// ClearIfUnchanged deletes a mark only if no write bumped it since it was read.
	ClearIfUnchanged(ctx context.Context, mark Mark) error
	// Oldest lists the oldest marks across every Product.
	Oldest(ctx context.Context, limit int) ([]Mark, error)
	// Postpone moves a Product's marks behind every other's.
	Postpone(ctx context.Context, productID string, until time.Time) error

	// RecordedModel returns the model the Product's store follows, if any.
	RecordedModel(ctx context.Context, productID string) (RecordedModel, error)
	SetModel(ctx context.Context, productID string, model RecordedModel) error
	SetReconciled(ctx context.Context, productID string, at time.Time) error
	// DueForReconcile names the Product rebuilt longest ago, if that was before
	// the given time; a Product never rebuilt comes first.
	DueForReconcile(ctx context.Context, before time.Time) (functional.Option[string], error)
	ProductExists(ctx context.Context, productID string) (bool, error)
}

// RecordedModel is the model a store follows and the hash it came from.
type RecordedModel struct {
	ID   string
	Hash string
}

// Locker serializes the syncs of one Product, across replicas.
type Locker interface {
	// TryLocked runs fn while holding the Product's sync lock, in a
	// transaction the Repository's calls in fn join. It reports false at once,
	// without running fn, when another holder has the lock.
	TryLocked(ctx context.Context, productID string, fn func(context.Context) error) (bool, error)
	// TrySweep runs fn unless another replica is sweeping.
	TrySweep(ctx context.Context, fn func(context.Context) error) (bool, error)
}

// Stores gives each Product its store, creating it on first use. It runs
// outside any transaction.
type Stores interface {
	StoreID(ctx context.Context, productID string) (string, error)
}

// Engines hands out the OpenFGA engine once it is ready.
type Engines interface {
	Get() (fga.Engine, error)
}
