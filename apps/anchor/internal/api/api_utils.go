package api

import (
	"math"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

const (
	legacyListDefaultLimit int64 = 50
	legacyListMaxLimit     int64 = 1000
)

// nextCursor turns the empty cursor of a last page into an absent property.
func nextCursor(cursor string) *string {
	if cursor == "" {
		return nil
	}
	return &cursor
}

// legacyListPagination maps the limit and offset query parameters of a
// deprecated GET list onto the pagination of its search. An omitted limit
// keeps the old default, and a limit above what a search accepts is capped.
func legacyListPagination(limit, offset int64) search.Pagination {
	if limit <= 0 {
		limit = legacyListDefaultLimit
	}
	return search.Pagination{
		Limit:  int32(min(limit, legacyListMaxLimit)),
		Offset: int32(min(max(offset, 0), math.MaxInt32)),
	}
}
