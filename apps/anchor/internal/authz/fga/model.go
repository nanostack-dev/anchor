package fga

import (
	"context"
	"fmt"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
)

// Relations is what an authorization model defines: the relations of each type.
type Relations map[string]map[string]bool

// RelationsOf lists the relations a model defines.
func RelationsOf(model *openfgav1.AuthorizationModel) Relations {
	relations := Relations{}
	for _, definition := range model.GetTypeDefinitions() {
		defined := map[string]bool{}
		for relation := range definition.GetRelations() {
			defined[relation] = true
		}
		relations[definition.GetType()] = defined
	}
	return relations
}

// Defines reports whether the tuple's relation exists on its object's type. A
// write of a tuple it does not define fails, so callers drop such tuples first.
func (r Relations) Defines(tuple Tuple) bool {
	return r[tuple.ObjectType()][tuple.Relation]
}

// WriteModel stores a new authorization model in the store and returns its ID.
func WriteModel(
	ctx context.Context, engine Engine, storeID string, model *openfgav1.AuthorizationModel,
) (string, error) {
	written, err := engine.WriteAuthorizationModel(ctx, &openfgav1.WriteAuthorizationModelRequest{
		StoreId:         storeID,
		SchemaVersion:   model.GetSchemaVersion(),
		TypeDefinitions: model.GetTypeDefinitions(),
		Conditions:      model.GetConditions(),
	})
	if err != nil {
		return "", fmt.Errorf("write authorization model: %w", err)
	}
	return written.GetAuthorizationModelId(), nil
}
