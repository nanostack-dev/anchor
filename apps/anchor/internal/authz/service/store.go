package service

import (
	"context"
	"fmt"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/model"
)

const (
	// OpenFGA's defaults for one Write call and one Read page.
	maxTuplesPerWrite = 100
	readPageSize      = 100

	onDuplicateIgnore = "ignore"
	onMissingIgnore   = "ignore"
)

// store is one product's OpenFGA store.
type store struct {
	engine engine.Engine
	id     string
}

// read returns every tuple matching the filter, across pages.
func (s store) read(ctx context.Context, filter *openfgav1.ReadRequestTupleKey) ([]model.Tuple, error) {
	var tuples []model.Tuple
	token := ""
	for {
		page, err := s.engine.Read(ctx, &openfgav1.ReadRequest{
			StoreId:           s.id,
			TupleKey:          filter,
			PageSize:          wrapperspb.Int32(readPageSize),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("read tuples: %w", err)
		}
		for _, tuple := range page.GetTuples() {
			key := tuple.GetKey()
			tuples = append(
				tuples,
				model.Tuple{Object: key.GetObject(), Relation: key.GetRelation(), User: key.GetUser()},
			)
		}
		if token = page.GetContinuationToken(); token == "" {
			return tuples, nil
		}
	}
}

// apply writes and deletes tuples in calls OpenFGA accepts. Both directions
// tolerate a tuple that is already there or already gone, so replaying the same
// changes after a crash is harmless.
func (s store) apply(ctx context.Context, modelID string, changes model.Changes) error {
	for _, chunk := range chunks(changes.Deletes) {
		if _, err := s.engine.Write(ctx, &openfgav1.WriteRequest{
			StoreId:              s.id,
			AuthorizationModelId: modelID,
			Deletes: &openfgav1.WriteRequestDeletes{
				TupleKeys: deleteKeys(chunk),
				OnMissing: onMissingIgnore,
			},
		}); err != nil {
			return fmt.Errorf("delete tuples: %w", err)
		}
	}
	for _, chunk := range chunks(changes.Writes) {
		if _, err := s.engine.Write(ctx, &openfgav1.WriteRequest{
			StoreId:              s.id,
			AuthorizationModelId: modelID,
			Writes: &openfgav1.WriteRequestWrites{
				TupleKeys:   writeKeys(chunk),
				OnDuplicate: onDuplicateIgnore,
			},
		}); err != nil {
			return fmt.Errorf("write tuples: %w", err)
		}
	}
	return nil
}

// latestModel returns the store's newest authorization model, if it has one.
func (s store) latestModel(ctx context.Context) (*openfgav1.AuthorizationModel, error) {
	models, err := s.engine.ReadAuthorizationModels(ctx, &openfgav1.ReadAuthorizationModelsRequest{
		StoreId:  s.id,
		PageSize: wrapperspb.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("read authorization models: %w", err)
	}
	if len(models.GetAuthorizationModels()) == 0 {
		return nil, nil
	}
	return models.GetAuthorizationModels()[0], nil
}

// ensureModel makes the built model the store's latest, writing it only when
// the latest one differs, and returns its ID.
func (s store) ensureModel(ctx context.Context, built model.Model) (string, error) {
	latest, err := s.latestModel(ctx)
	if err != nil {
		return "", err
	}
	if latest != nil && sameModel(latest, built.Proto) {
		return latest.GetId(), nil
	}
	written, err := s.engine.WriteAuthorizationModel(ctx, &openfgav1.WriteAuthorizationModelRequest{
		StoreId:         s.id,
		TypeDefinitions: built.Proto.GetTypeDefinitions(),
		SchemaVersion:   built.Proto.GetSchemaVersion(),
		Conditions:      built.Proto.GetConditions(),
	})
	if err != nil {
		return "", fmt.Errorf("write authorization model: %w", err)
	}
	return written.GetAuthorizationModelId(), nil
}

// holds asks which of the relations the user has on the object, in one call.
func (s store) holds(ctx context.Context, modelID, user, object string, relations []string) (map[string]bool, error) {
	held := make(map[string]bool, len(relations))
	if len(relations) == 0 {
		return held, nil
	}
	checks := make([]*openfgav1.BatchCheckItem, len(relations))
	for i, relation := range relations {
		checks[i] = &openfgav1.BatchCheckItem{
			TupleKey:      &openfgav1.CheckRequestTupleKey{User: user, Relation: relation, Object: object},
			CorrelationId: fmt.Sprintf("c%d", i),
		}
	}
	response, err := s.engine.BatchCheck(ctx, &openfgav1.BatchCheckRequest{
		StoreId:              s.id,
		AuthorizationModelId: modelID,
		Checks:               checks,
	})
	if err != nil {
		return nil, fmt.Errorf("batch check: %w", err)
	}
	for i, relation := range relations {
		result := response.GetResult()[checks[i].GetCorrelationId()]
		if result.GetError() != nil {
			return nil, fmt.Errorf("check %s: %s", relation, result.GetError().GetMessage())
		}
		held[relation] = result.GetAllowed()
	}
	return held, nil
}

func sameModel(a, b *openfgav1.AuthorizationModel) bool {
	if a.GetSchemaVersion() != b.GetSchemaVersion() || len(a.GetTypeDefinitions()) != len(b.GetTypeDefinitions()) {
		return false
	}
	for i := range a.GetTypeDefinitions() {
		if !proto.Equal(a.GetTypeDefinitions()[i], b.GetTypeDefinitions()[i]) {
			return false
		}
	}
	return true
}

func chunks(tuples []model.Tuple) [][]model.Tuple {
	var out [][]model.Tuple
	for start := 0; start < len(tuples); start += maxTuplesPerWrite {
		out = append(out, tuples[start:min(start+maxTuplesPerWrite, len(tuples))])
	}
	return out
}

func writeKeys(tuples []model.Tuple) []*openfgav1.TupleKey {
	keys := make([]*openfgav1.TupleKey, len(tuples))
	for i, t := range tuples {
		keys[i] = &openfgav1.TupleKey{Object: t.Object, Relation: t.Relation, User: t.User}
	}
	return keys
}

func deleteKeys(tuples []model.Tuple) []*openfgav1.TupleKeyWithoutCondition {
	keys := make([]*openfgav1.TupleKeyWithoutCondition, len(tuples))
	for i, t := range tuples {
		keys[i] = &openfgav1.TupleKeyWithoutCondition{Object: t.Object, Relation: t.Relation, User: t.User}
	}
	return keys
}
