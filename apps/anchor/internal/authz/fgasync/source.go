// Package fgasync keeps each Product's OpenFGA store in step with Anchor's
// tables (ADR-0022 E5). The tables stay the source of truth; a store holds the
// tuples they imply.
//
// A module syncs its slice of the tables by implementing Source: which
// events make which keys stale, which keys exist, the tuples a key implies,
// and the tuples it owns in the store. The syncer does the rest, once for
// every source:
//
//   - Mark. A write that changes access leaves a mark in its own transaction
//     (Marker).
//   - Replay. Right after the request commits, its marks are synced before the
//     response is written, so the next request already sees the change.
//   - Sweep. Every few seconds, marks a replay missed are synced.
//   - Reconcile. Every Product is rebuilt in full on a slow cycle, which also
//     deletes every tuple no source claims.
//
// A sync never applies an event. It reads what the key implies now, reads what
// the store holds for it, and writes the difference, so a sync that runs late,
// twice or out of order is harmless.
package fgasync

import (
	"context"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"

	"anchor/internal/authz/fga"
	"anchor/internal/events"
)

// EveryKey is the stale key that makes every key of the source stale: for
// a change that reaches many keys without naming them, such as a cascade.
const EveryKey = "*"

// Source syncs one slice of Anchor's tables into each Product's store.
// A key is the unit it syncs, such as one Organization or one role.
//
// Sources must own disjoint tuples: two sources, or two keys, whose
// Stored overlap would delete each other's tuples. Stored must return every
// tuple Desired can return, or the syncer writes it again on every sync and
// never deletes it.
type Source interface {
	// Name identifies the source in marks. It must stay stable: renaming
	// it orphans the marks that wait under the old name.
	Name() string
	// Stale lists the keys an event makes stale, or none when the event is not
	// about this source. It runs inside the event's transaction; an error
	// rolls the write back.
	Stale(event events.Event) ([]string, error)
	// Keys lists every key the Product holds now, for a full sync.
	Keys(ctx context.Context, productID string) ([]string, error)
	// Desired lists the tuples the key implies now. A key that no longer
	// exists implies none. A tuple whose relation the model lacks is dropped.
	Desired(ctx context.Context, productID, key string) ([]fga.Tuple, error)
	// Stored lists the tuples the key owns in the store.
	Stored(ctx context.Context, store fga.Reader, key string) ([]fga.Tuple, error)
}

// ModelSource gives each Product its authorization model.
type ModelSource interface {
	// Model reads what the Product's model compiles from.
	Model(ctx context.Context, productID string) (Model, error)
}

// ModelChanges reports whether an event changes the model. Such an event makes
// the whole Product stale: every source resyncs under the new model.
type ModelChanges func(event events.Event) bool

// Model is a Product's authorization model before it is compiled. The syncer
// compiles it only for a hash it has not compiled before.
type Model struct {
	// Hash changes whenever the compiled model would.
	Hash    string
	Compile func() (*openfgav1.AuthorizationModel, error)
}

// Scope is what a mark makes stale: one key of one source, every key of a
// source (Key is EveryKey), or the whole Product (Source is empty).
type Scope struct {
	ProductID string
	Source    string
	Key       string
}

// WholeProduct is the scope of a model change: every source, every key.
func WholeProduct(productID string) Scope { return Scope{ProductID: productID} }

func (s Scope) IsWholeProduct() bool { return s.Source == "" }

func (s Scope) IsEveryKey() bool { return s.Source != "" && s.Key == EveryKey }

// covers reports whether syncing s also syncs other.
func (s Scope) covers(other Scope) bool {
	switch {
	case s.ProductID != other.ProductID:
		return false
	case s.IsWholeProduct():
		return true
	case s.IsEveryKey():
		return other.Source == s.Source
	default:
		return s == other
	}
}

// Mark is a stale scope and the revision its last write gave it.
type Mark struct {
	Scope
	Revision int64
}
