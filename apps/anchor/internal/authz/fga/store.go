package fga

import (
	"context"
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	// OpenFGA accepts at most this many writes and deletes in one request.
	writeBatch = 100
	readPage   = 100
)

// Engine is OpenFGA's own service API, in-process or remote.
type Engine = openfgav1.OpenFGAServiceServer

// Filter selects tuples to read. Object is required unless the whole store is
// read; it may be a type alone, "type:", when User is set. Empty fields match
// anything.
type Filter struct {
	Object   string
	Relation string
	User     string
}

// Reader reads tuples from a store.
type Reader interface {
	// Read returns every tuple the filter matches, across all pages.
	Read(ctx context.Context, filter Filter) ([]Tuple, error)
}

// Store is one OpenFGA store, written under one authorization model.
type Store struct {
	engine  Engine
	id      string
	modelID string
}

func NewStore(engine Engine, storeID, modelID string) Store {
	return Store{engine: engine, id: storeID, modelID: modelID}
}

func (s Store) ID() string { return s.id }

func (s Store) ModelID() string { return s.modelID }

func (s Store) Read(ctx context.Context, filter Filter) ([]Tuple, error) {
	var key *openfgav1.ReadRequestTupleKey
	if filter != (Filter{}) {
		key = &openfgav1.ReadRequestTupleKey{Object: filter.Object, Relation: filter.Relation, User: filter.User}
	}
	var tuples []Tuple
	token := ""
	for {
		resp, err := s.engine.Read(ctx, &openfgav1.ReadRequest{
			StoreId:           s.id,
			TupleKey:          key,
			PageSize:          wrapperspb.Int32(readPage),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("read tuples: %w", err)
		}
		for _, t := range resp.GetTuples() {
			k := t.GetKey()
			tuples = append(tuples, Tuple{Object: k.GetObject(), Relation: k.GetRelation(), User: k.GetUser()})
		}
		if token = resp.GetContinuationToken(); token == "" {
			return tuples, nil
		}
	}
}

// Apply writes the changes in batches, deletes first, so a run cut short never
// leaves a subject with both its old and its new grants. A tuple already there,
// or already gone, is not an error: a run cut short is replayed whole.
func (s Store) Apply(ctx context.Context, changes Changes) error {
	for !changes.IsEmpty() {
		deletes := take(&changes.Deletes, writeBatch)
		writes := take(&changes.Writes, writeBatch-len(deletes))
		request := &openfgav1.WriteRequest{StoreId: s.id, AuthorizationModelId: s.modelID}
		if len(writes) > 0 {
			request.Writes = &openfgav1.WriteRequestWrites{
				TupleKeys:   functional.Slice(writes).Map(toTupleKey),
				OnDuplicate: "ignore",
			}
		}
		if len(deletes) > 0 {
			request.Deletes = &openfgav1.WriteRequestDeletes{
				TupleKeys: functional.Slice(deletes).Map(toTupleKeyWithoutCondition),
				OnMissing: "ignore",
			}
		}
		if _, err := s.engine.Write(ctx, request); err != nil {
			return fmt.Errorf("write tuples: %w", err)
		}
	}
	return nil
}

// Check answers whether user holds relation on object under the store's model.
func (s Store) Check(ctx context.Context, user, relation, object string) (bool, error) {
	resp, err := s.engine.Check(ctx, &openfgav1.CheckRequest{
		StoreId:              s.id,
		AuthorizationModelId: s.modelID,
		TupleKey:             &openfgav1.CheckRequestTupleKey{User: user, Relation: relation, Object: object},
	})
	if err != nil {
		return false, fmt.Errorf("check: %w", err)
	}
	return resp.GetAllowed(), nil
}

func take(tuples *[]Tuple, n int) []Tuple {
	n = min(n, len(*tuples))
	batch := (*tuples)[:n]
	*tuples = (*tuples)[n:]
	return batch
}

func toTupleKey(t Tuple) *openfgav1.TupleKey {
	return &openfgav1.TupleKey{Object: t.Object, Relation: t.Relation, User: t.User}
}

func toTupleKeyWithoutCondition(t Tuple) *openfgav1.TupleKeyWithoutCondition {
	return &openfgav1.TupleKeyWithoutCondition{Object: t.Object, Relation: t.Relation, User: t.User}
}
