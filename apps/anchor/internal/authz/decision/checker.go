// Package decision answers access decisions from each Product's OpenFGA store
// (ADR-0021; ADR-0022 G4, PR 7). Postgres still resolves who the subject is;
// what it holds comes from the store.
package decision

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"golang.org/x/sync/singleflight"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/engine"
	"anchor/internal/authz/fga"
	"anchor/internal/authz/fgasync"
	"anchor/internal/authz/model"
	"anchor/internal/authz/store"
)

const (
	// checkTimeout bounds the checks of one decision, however many actions it
	// names.
	checkTimeout = 5 * time.Second
	// buildTimeout bounds the first build of a Product's store, which runs
	// detached from the request that asked for it.
	buildTimeout = 30 * time.Second
)

// errNotBuilt reports a Product whose store is still not built after a build.
var errNotBuilt = errors.New("authz store not built")

// Rebuilder fills a Product's store from its tables in full.
type Rebuilder interface {
	Reconcile(ctx context.Context, productID string) error
}

// Checker reads each Product's store as it is: the write that changed access
// already reached it after its commit, and decisions never look for marks
// (E5).
type Checker struct {
	stores    store.Stores
	rebuilder Rebuilder
	builds    singleflight.Group
}

func NewChecker(stores store.Stores, rebuilder Rebuilder) *Checker {
	return &Checker{stores: stores, rebuilder: rebuilder}
}

// Holds checks every action on the object. An unregistered resource is
// checked as an object of its type that exists for the check only, with its
// parent given as a contextual tuple (ADR-0022 B1). A permission the model
// doesn't define on the object's type is asked on the check's fallback
// object, when it names one, or is not held; neither is anything on a type the
// model lacks, or under a parent its parent relation refuses.
// Anything OpenFGA can't answer, or can't answer in time, fails with
// accessservice.ErrPermissionStoreUnavailable.
func (c *Checker) Holds(ctx context.Context, check accessservice.PermissionCheck) (map[string]bool, error) {
	held := make(map[string]bool, len(check.Actions))
	if len(check.Actions) == 0 {
		return held, nil
	}
	opened, err := c.store(ctx, check.ProductID)
	if err != nil {
		return nil, err
	}
	subject := model.User(check.ProductUserID)
	if check.APIKeyID != "" {
		subject = model.APIKey(check.APIKeyID)
	}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	remaining := check.Actions
	for _, on := range []*accessservice.CheckedObject{&check.Object, check.Fallback} {
		if on == nil || len(remaining) == 0 {
			continue
		}
		object, contextual := checkedObject(*on)
		split := functional.Slice(remaining).Partition(func(action string) bool {
			return opened.Relations.Defines(fga.Tuple{Object: object, Relation: model.CanRelation(action)})
		})
		asked := split.First
		remaining = split.Second
		tuples := functional.Slice(asked).Map(func(action string) fga.Tuple {
			return fga.Tuple{Object: object, Relation: model.CanRelation(action), User: subject}
		})
		answers, holdsErr := opened.Holds(ctx, tuples, contextual...)
		if holdsErr != nil {
			return nil, unavailable(holdsErr)
		}
		for i, action := range asked {
			held[action] = held[action] || answers[i]
		}
	}
	return held, nil
}

// checkedObject names the checked object in the model, and the tuples that
// hold for its check only.
func checkedObject(object accessservice.CheckedObject) (string, []fga.Tuple) {
	if object.Parent == nil {
		return objectRef(object), nil
	}
	unregistered := model.UnregisteredResource(object.Type)
	return unregistered, []fga.Tuple{
		{Object: unregistered, Relation: model.RelationParent, User: objectRef(*object.Parent)},
	}
}

func objectRef(object accessservice.CheckedObject) string {
	if object.Type == model.TypeOrganization {
		return model.Organization(object.ID)
	}
	return model.ResourceObject(object.Type, object.ID)
}

// store returns the Product's store once a full rebuild filled it. The first
// decision for a Product never rebuilt builds it, once per process, while the
// other decisions for it wait.
func (c *Checker) store(ctx context.Context, productID string) (store.Opened, error) {
	found, err := c.stores.Open(ctx, productID)
	if errors.Is(err, engine.ErrNotReady) {
		return store.Opened{}, unavailable(err)
	}
	if err != nil {
		return store.Opened{}, err
	}
	if opened, ok := found.Get(); ok && opened.Rebuilt {
		return opened, nil
	}
	result := c.builds.DoChan(productID, func() (any, error) {
		buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), buildTimeout)
		defer cancel()
		if buildErr := c.rebuilder.Reconcile(buildCtx, productID); buildErr != nil {
			return nil, buildErr
		}
		return c.stores.Open(buildCtx, productID)
	})
	select {
	case <-ctx.Done():
		return store.Opened{}, ctx.Err()
	case outcome := <-result:
		return built(outcome)
	}
}

func built(outcome singleflight.Result) (store.Opened, error) {
	if outcome.Err != nil {
		if errors.Is(outcome.Err, engine.ErrNotReady) || errors.Is(outcome.Err, fgasync.ErrLockBusy) ||
			errors.Is(outcome.Err, context.DeadlineExceeded) {
			return store.Opened{}, unavailable(outcome.Err)
		}
		return store.Opened{}, outcome.Err
	}
	found, _ := outcome.Val.(functional.Option[store.Opened])
	if opened, ok := found.Get(); ok && opened.Rebuilt {
		return opened, nil
	}
	return store.Opened{}, errNotBuilt
}

func unavailable(err error) error {
	return fmt.Errorf("%w: %w", accessservice.ErrPermissionStoreUnavailable, err)
}
