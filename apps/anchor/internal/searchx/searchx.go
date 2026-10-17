// Package searchx holds the small pieces every search shares that the
// framework search package leaves to the application.
package searchx

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
)

// IsClientError reports a mistake in the request that a search repository
// found: a cursor that does not continue this search, one combined with an
// offset, or a sort field the search cannot order by. The transport layer
// answers each with a 400 (INVALID_CURSOR, INVALID_SORT), so a service must
// pass it through unchanged instead of collapsing it into an unexpected error.
func IsClientError(err error) bool {
	return errors.Is(err, search.ErrInvalidCursor) ||
		errors.Is(err, search.ErrCursorWithOffset) ||
		errors.Is(err, search.ErrUnknownSort)
}

// Sort picks the requested sort, or the resource's documented default order
// when the client sent no sort_by. A client that names a field without a
// direction gets ascending, as search.Request.WithSort always did; only an
// omitted sort_by takes the resource's default direction.
func Sort[S ~string](
	field *S, direction *search.SortDirection, defaultField S, defaultDirection search.SortDirection,
) (*S, *search.SortDirection) {
	if field != nil {
		return field, direction
	}
	if direction == nil {
		direction = &defaultDirection
	}
	return &defaultField, direction
}

// ValidateRequest validates a search input: the pagination cursor, the struct
// tags, then the sort. The cursor goes first because the struct tag on the
// cursor length would otherwise answer a generic validation error where the
// contract promises INVALID_CURSOR, and because a cursor combined with an offset
// is refused here, before any scope check can answer an empty page instead.
//
// The request body of a search is not checked against the OpenAPI enums, so a
// sort_by outside the resource's sortable fields would otherwise be ignored and
// the default order served in its place. Allowed lists every sort field the
// resource maps.
func ValidateRequest[F any, S ~string](input any, request search.Request[F, S], allowed ...S) error {
	if err := request.Pagination.Validate(); err != nil {
		return err
	}
	if err := validate.ValidateStruct(input); err != nil {
		return err
	}
	for _, sort := range request.Sort {
		if !slices.Contains(allowed, sort.Field) {
			return notOneOf("sort_by", string(sort.Field), allowedStrings(allowed))
		}
	}
	return nil
}

// ValidateOneOf reports a filter value outside the closed set the contract
// names, for a set the struct tags cannot spell.
func ValidateOneOf(field string, values []string, allowed []string) error {
	for _, value := range values {
		if !slices.Contains(allowed, value) {
			return notOneOf(field, value, allowed)
		}
	}
	return nil
}

func allowedStrings[S ~string](allowed []S) []string {
	out := make([]string, len(allowed))
	for i, value := range allowed {
		out[i] = string(value)
	}
	return out
}

func notOneOf(field, value string, allowed []string) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:    "VALIDATION_ERROR",
		Message: field + " must be one of: " + strings.Join(allowed, ", "),
		Metadata: map[string]any{
			"field": field,
			"rule":  "oneof",
			"param": strings.Join(allowed, " "),
			"value": value,
		},
	}}, http.StatusBadRequest)
}
