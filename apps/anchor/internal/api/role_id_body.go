package api

import (
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/oapi-codegen/nullable"
)

// roleIDField names the role_id body field in an error.
const roleIDField = "role_id"

// requiredRoleID reads a role_id the contract requires and allows to be null:
// null means no role. The request validator skips bodies, so a missing field is
// refused here, in the shape the validate tags give the other fields.
func requiredRoleID(field nullable.Nullable[string]) (*string, error) {
	if !field.IsSpecified() {
		return nil, fault.NewWithDetails([]fault.Detail{{
			Code:     "VALIDATION_ERROR",
			Message:  "role_id is a required field; send null for no role",
			Field:    roleIDField,
			Metadata: map[string]any{"rule": "required"},
		}}, http.StatusBadRequest)
	}
	if field.IsNull() {
		return nil, nil
	}
	roleID := field.MustGet()
	return &roleID, nil
}
