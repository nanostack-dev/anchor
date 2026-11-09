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

// orEmpty reads an optional list property: a property that was not sent filters
// nothing, like an empty list.
func orEmpty[T any](list *[]T) []T {
	if list == nil {
		return nil
	}
	return *list
}

// requireNotBlank refuses a blank term of a substring filter. A blank term
// matches every row, so a client that sends one by mistake would read the whole
// set instead of the rows it named.
func requireNotBlank(field string, terms []string) error {
	for _, term := range terms {
		if strings.TrimSpace(term) != "" {
			continue
		}
		return fault.NewWithDetails([]fault.Detail{{
			Code:    "VALIDATION_ERROR",
			Message: field + " must not contain a blank term",
			Metadata: map[string]any{
				"field": field,
				"rule":  "notblank",
				"value": term,
			},
		}}, http.StatusBadRequest)
	}
	return nil
}
