// Package searchx holds the small pieces every search shares that the
// framework search package leaves to the application.
package searchx

import (
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

// IsCursorError reports a client mistake in the pagination cursor: a cursor
// that does not continue this search, or one combined with an offset. The
// transport layer answers it with 400 INVALID_CURSOR, so a service must pass
// it through instead of collapsing it into an unexpected error.
func IsCursorError(err error) bool {
	return errors.Is(err, search.ErrInvalidCursor) || errors.Is(err, search.ErrCursorWithOffset)
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
