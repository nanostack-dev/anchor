package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/engine"
	"anchor/internal/authz/model"
	"anchor/internal/authz/repository"
)

// checkBatch is OpenFGA's default limit of checks in one BatchCheck.
const checkBatch = 50

var (
	// errNoModel reports a Product whose store still has no model after a build.
	errNoModel = errors.New("authz store has no model")
	// errCheckFailed reports an item OpenFGA could not answer. An input error,
	// such as a permission the model doesn't know, answers not held instead.
	errCheckFailed = errors.New("authz check failed")
)

// Checker answers access decisions from each Product's OpenFGA store
// (ADR-0021; ADR-0022 G4, PR 7). It reads the store as it is: the write that
// changed access already reached it after its commit, and decisions never look
// for marks (E5).
type Checker struct {
	engines engine.Provider
	repo    repository.Repository
	syncer  Syncer
}

func NewChecker(engines engine.Provider, repo repository.Repository, syncer Syncer) accessservice.PermissionChecker {
	return &Checker{engines: engines, repo: repo, syncer: syncer}
}

// Holds checks every action on the identity's Organization, in BatchChecks of
// at most checkBatch items.
// An action the model doesn't know is not held; an item OpenFGA fails on
// fails the decision.
func (c *Checker) Holds(ctx context.Context, check accessservice.PermissionCheck) (map[string]bool, error) {
	held := make(map[string]bool, len(check.Actions))
	if len(check.Actions) == 0 {
		return held, nil
	}
	openfga, err := c.engines.Get()
	if err != nil {
		return nil, err
	}
	stored, err := c.store(ctx, check.ProductID)
	if err != nil {
		return nil, err
	}
	subject := model.User(check.ProductUserID)
	if check.APIKeyID != "" {
		subject = model.APIKey(check.APIKeyID)
	}
	for start := 0; start < len(check.Actions); start += checkBatch {
		actions := check.Actions[start:min(start+checkBatch, len(check.Actions))]
		if err = c.holdsBatch(ctx, openfga, stored, subject, check.OrganizationID, actions, held); err != nil {
			return nil, err
		}
	}
	return held, nil
}

// holdsBatch answers at most checkBatch actions in one BatchCheck.
func (c *Checker) holdsBatch(
	ctx context.Context, openfga engine.Engine, stored repository.Store,
	subject, organizationID string, actions []string, held map[string]bool,
) error {
	checks := make([]*openfgav1.BatchCheckItem, len(actions))
	for i, action := range actions {
		checks[i] = &openfgav1.BatchCheckItem{
			CorrelationId: strconv.Itoa(i),
			TupleKey: &openfgav1.CheckRequestTupleKey{
				User:     subject,
				Relation: model.CanRelation(action),
				Object:   model.Organization(organizationID),
			},
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
		if !ok || result.GetError().GetInternalError() != openfgav1.InternalErrorCode_no_internal_error {
			return fmt.Errorf("%w: %s", errCheckFailed, result.GetError().GetMessage())
		}
		held[action] = held[action] || result.GetAllowed()
	}
	return nil
}

// store returns the Product's store and model, building both on the first
// decision for a Product nothing has synced yet.
func (c *Checker) store(ctx context.Context, productID string) (repository.Store, error) {
	found, err := c.repo.FindStore(ctx, productID)
	if err != nil {
		return repository.Store{}, err
	}
	if stored, ok := found.Get(); ok && stored.ModelID != "" {
		return stored, nil
	}
	if err = c.syncer.SyncProduct(ctx, productID); err != nil {
		return repository.Store{}, err
	}
	if found, err = c.repo.FindStore(ctx, productID); err != nil {
		return repository.Store{}, err
	}
	if stored, ok := found.Get(); ok && stored.ModelID != "" {
		return stored, nil
	}
	return repository.Store{}, errNoModel
}
