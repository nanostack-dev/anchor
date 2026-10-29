package fga

import (
	"context"
	"fmt"
	"strings"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
)

// Relations is what an authorization model defines: the relations of each
// type, and the user types each relation takes directly.
type Relations map[string]map[string][]*openfgav1.RelationReference

// RelationsOf lists the relations a model defines.
func RelationsOf(model *openfgav1.AuthorizationModel) Relations {
	relations := Relations{}
	for _, definition := range model.GetTypeDefinitions() {
		defined := map[string][]*openfgav1.RelationReference{}
		direct := definition.GetMetadata().GetRelations()
		for relation := range definition.GetRelations() {
			defined[relation] = direct[relation].GetDirectlyRelatedUserTypes()
		}
		relations[definition.GetType()] = defined
	}
	return relations
}

// Defines reports whether the tuple's relation exists on its object's type,
// so it can be checked.
func (r Relations) Defines(tuple Tuple) bool {
	_, defined := r[tuple.ObjectType()][tuple.Relation]
	return defined
}

// Accepts reports whether the model lets the tuple be written: its relation
// takes the tuple's user type directly. A write of a tuple it does not accept
// fails, so callers drop such tuples first.
func (r Relations) Accepts(tuple Tuple) bool {
	userType, userID := TypeOf(tuple.User), IDOf(tuple.User)
	userRelation := ""
	if before, after, found := strings.Cut(tuple.User, "#"); found {
		userID, userRelation = IDOf(before), after
	}
	for _, direct := range r[tuple.ObjectType()][tuple.Relation] {
		if direct.GetType() != userType {
			continue
		}
		switch {
		case direct.GetWildcard() != nil:
			if userID == "*" && userRelation == "" {
				return true
			}
		case direct.GetRelation() != "":
			if userRelation == direct.GetRelation() {
				return true
			}
		case userID != "*" && userRelation == "":
			return true
		}
	}
	return false
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
