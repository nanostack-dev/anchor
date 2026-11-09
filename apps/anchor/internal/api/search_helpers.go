package api

import (
	"net/http"
	"slices"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
)

// requireOneOf refuses a filter value outside the closed set the contract
// names. The request body of a search is not checked against the OpenAPI
// enums, and a generated builder takes any value of a named type, so an
// unknown status would otherwise read as a filter that matches no row.
func requireOneOf[T ~string](field string, values []T, allowed ...T) error {
	for _, value := range values {
		if slices.Contains(allowed, value) {
			continue
		}
		names := make([]string, len(allowed))
		for i, name := range allowed {
			names[i] = string(name)
		}
		return fault.NewWithDetails([]fault.Detail{{
			Code:    "VALIDATION_ERROR",
			Message: field + " must be one of: " + strings.Join(names, ", "),
			Metadata: map[string]any{
				"field": field,
				"rule":  "oneof",
				"param": strings.Join(names, " "),
				"value": string(value),
			},
		}}, http.StatusBadRequest)
	}
	return nil
}
