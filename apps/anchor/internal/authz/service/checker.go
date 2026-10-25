package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"golang.org/x/sync/singleflight"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/engine"
	"anchor/internal/authz/model"
	"anchor/internal/authz/repository"
)

const (
	// checkBatch is OpenFGA's default limit of checks in one BatchCheck.
	checkBatch = 50
	// checkTimeout bounds the checks of one decision, however many actions it
	// names.
	checkTimeout = 5 * time.Second
	// buildTimeout bounds the first build of a Product's store, which runs
	// detached from the request that asked for it.
	buildTimeout = 30 * time.Second
)

var (
	// errNoModel reports a Product whose store is still not built after a build.
	errNoModel = errors.New("authz store has no model")
	// errCheckFailed reports an item OpenFGA could not answer.
	errCheckFailed = errors.New("authz check failed")
)

// Checker answers access decisions from each Product's OpenFGA store
// (ADR-0021; ADR-0022 G4, PR 7 and PR 13). It reads the store as it is: the
// write that changed access already reached it after its commit, and
// decisions never look for marks (E5).
type Checker struct {
	engines engine.Provider
	repo    repository.Repository
	syncer  Syncer
	builds  singleflight.Group
	// types holds, per model ID, what each type of the model defines; a
	// model never changes once written.
	types sync.Map
}

// modelType is what one type of a model defines: its relations, and the
// types its parent relation accepts.
type modelType struct {
	relations map[string]bool
	parents   map[string]bool
}

func NewChecker(engines engine.Provider, repo repository.Repository, syncer Syncer) accessservice.PermissionChecker {
	return &Checker{engines: engines, repo: repo, syncer: syncer}
}

// Holds checks every action on the object, in BatchChecks of at most
// checkBatch items. An unregistered resource is checked as an object of its
// type that exists for the check only, with its parent given as a contextual
// tuple. A permission the model doesn't define on the object's type is not
// held, and is not asked; neither is anything on a type or a parent the model
// lacks.
// Anything OpenFGA can't answer, or can't answer in time, fails the decision
// with accessservice.ErrPermissionStoreUnavailable.
func (c *Checker) Holds(ctx context.Context, check accessservice.PermissionCheck) (map[string]bool, error) {
	held := make(map[string]bool, len(check.Actions))
	if len(check.Actions) == 0 {
		return held, nil
	}
	openfga, err := c.engines.Get()
	if err != nil {
		return nil, unavailable(err)
	}
	stored, err := c.store(ctx, check.ProductID)
	if err != nil {
		return nil, err
	}
	subject := model.User(check.ProductUserID)
	if check.APIKeyID != "" {
		subject = model.APIKey(check.APIKeyID)
	}
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	types, err := c.modelTypes(ctx, openfga, stored)
	if err != nil {
		return nil, unavailable(err)
	}
	on, ok := checkTarget(check.Object, types)
	if !ok {
		return held, nil
	}
	defined := types[model.ResourceType(check.Object.Type)].relations
	declared := functional.Slice(check.Actions).Filter(func(action string) bool {
		return defined[model.CanRelation(action)]
	})
	for start := 0; start < len(declared); start += checkBatch {
		actions := declared[start:min(start+checkBatch, len(declared))]
		if err = c.holdsBatch(ctx, openfga, stored, subject, on, actions, held); err != nil {
			return nil, unavailable(err)
		}
	}
	return held, nil
}

// checkedTarget is the object a BatchCheck asks about, and the tuples it holds for
// that check only.
type checkedTarget struct {
	object     string
	contextual *openfgav1.ContextualTupleKeys
}

// checkTarget names the checked object in the model. It refuses a type the
// model lacks and a contextual parent its parent relation would refuse, which
// OpenFGA would answer with an error rather than a denial.
func checkTarget(object accessservice.CheckedObject, types map[string]modelType) (checkedTarget, bool) {
	objectType, defined := types[model.ResourceType(object.Type)]
	if !defined {
		return checkedTarget{}, false
	}
	if object.Parent == nil {
		return checkedTarget{object: checkedObject(object)}, true
	}
	if !objectType.parents[model.ResourceType(object.Parent.Type)] {
		return checkedTarget{}, false
	}
	unregistered := model.UnregisteredResource(object.Type)
	return checkedTarget{object: unregistered, contextual: &openfgav1.ContextualTupleKeys{
		TupleKeys: []*openfgav1.TupleKey{{
			Object: unregistered, Relation: model.RelationParent, User: checkedObject(*object.Parent),
		}},
	}}, true
}

func checkedObject(object accessservice.CheckedObject) string {
	if object.Type == model.TypeOrganization {
		return model.Organization(object.ID)
	}
	return model.ResourceObject(object.Type, object.ID)
}

// holdsBatch answers at most checkBatch actions in one BatchCheck.
func (c *Checker) holdsBatch(
	ctx context.Context, openfga engine.Engine, stored repository.Store,
	subject string, on checkedTarget, actions []string, held map[string]bool,
) error {
	checks := make([]*openfgav1.BatchCheckItem, len(actions))
	for i, action := range actions {
		checks[i] = &openfgav1.BatchCheckItem{
			CorrelationId: strconv.Itoa(i),
			TupleKey: &openfgav1.CheckRequestTupleKey{
				User:     subject,
				Relation: model.CanRelation(action),
				Object:   on.object,
			},
			ContextualTuples: on.contextual,
		}
	}
	resp, err := openfga.BatchCheck(ctx, &openfgav1.BatchCheckRequest{
		StoreId: stored.StoreID, AuthorizationModelId: stored.ModelID, Checks: checks,
	})
	if err != nil {
		return err
	}
	for i, action := range actions {
		result, ok := resp.GetResult()[strconv.Itoa(i)]
		switch {
		case !ok:
			return fmt.Errorf("%w: no result for %s", errCheckFailed, action)
		case result.GetError() != nil:
			return fmt.Errorf("%w: %s", errCheckFailed, result.GetError().GetMessage())
		default:
			held[action] = held[action] || result.GetAllowed()
		}
	}
	return nil
}

// modelTypes lists what each type of the model defines, so a permission the
// catalog doesn't define on a type is answered not held without asking:
// OpenFGA reports it with the same code as throttling.
func (c *Checker) modelTypes(
	ctx context.Context, openfga engine.Engine, stored repository.Store,
) (map[string]modelType, error) {
	if cached, ok := c.types.Load(stored.ModelID); ok {
		if types, isMap := cached.(map[string]modelType); isMap {
			return types, nil
		}
	}
	resp, err := openfga.ReadAuthorizationModel(ctx, &openfgav1.ReadAuthorizationModelRequest{
		StoreId: stored.StoreID, Id: stored.ModelID,
	})
	if err != nil {
		return nil, err
	}
	types := map[string]modelType{}
	for _, definition := range resp.GetAuthorizationModel().GetTypeDefinitions() {
		defined := modelType{relations: map[string]bool{}, parents: map[string]bool{}}
		for relation := range definition.GetRelations() {
			defined.relations[relation] = true
		}
		parent := definition.GetMetadata().GetRelations()[model.RelationParent]
		for _, related := range parent.GetDirectlyRelatedUserTypes() {
			defined.parents[related.GetType()] = true
		}
		types[definition.GetType()] = defined
	}
	c.types.Store(stored.ModelID, types)
	return types, nil
}

// store returns the Product's store once a full rebuild has filled it. Until
// then the store may hold only the Organizations written since, and would deny
// the others. The first decision for a Product never rebuilt builds it, once
// per process, while the other decisions for it wait.
func (c *Checker) store(ctx context.Context, productID string) (repository.Store, error) {
	found, err := c.repo.FindStore(ctx, productID)
	if err != nil {
		return repository.Store{}, err
	}
	if stored, ok := found.Get(); ok && built(stored) {
		return stored, nil
	}
	result := c.builds.DoChan(productID, func() (any, error) {
		buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), buildTimeout)
		defer cancel()
		if buildErr := c.syncer.Reconcile(buildCtx, productID); buildErr != nil {
			return nil, buildErr
		}
		return c.repo.FindStore(buildCtx, productID)
	})
	select {
	case <-ctx.Done():
		return repository.Store{}, ctx.Err()
	case outcome := <-result:
		return builtStore(outcome)
	}
}

func builtStore(outcome singleflight.Result) (repository.Store, error) {
	if outcome.Err != nil {
		if errors.Is(outcome.Err, engine.ErrNotReady) || errors.Is(outcome.Err, errLockBusy) ||
			errors.Is(outcome.Err, context.DeadlineExceeded) {
			return repository.Store{}, unavailable(outcome.Err)
		}
		return repository.Store{}, outcome.Err
	}
	found, _ := outcome.Val.(functional.Option[repository.Store])
	if stored, ok := found.Get(); ok && built(stored) {
		return stored, nil
	}
	return repository.Store{}, errNoModel
}

func built(stored repository.Store) bool { return stored.ModelID != "" && stored.ReconciledAt != nil }

func unavailable(err error) error {
	return fmt.Errorf("%w: %w", accessservice.ErrPermissionStoreUnavailable, err)
}
