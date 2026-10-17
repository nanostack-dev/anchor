package api

import (
	"math"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

const (
	legacyListDefaultLimit int64 = 50
	legacyListPageSize     int64 = 1000
)

// nextCursor turns the empty cursor of a last page into an absent property.
func nextCursor(cursor string) *string {
	if cursor == "" {
		return nil
	}
	return &cursor
}

// legacyListPage reads the page a deprecated GET list asked for, whatever its
// limit. The old lists took any limit, a search takes at most
// legacyListPageSize, so a larger page is assembled from consecutive searches:
// the first starts at the offset and each next one continues by cursor. Total
// is the first search's, and the page ends early only when the rows do.
func legacyListPage[T any](
	limit, offset int64,
	fetch func(pagination search.Pagination) (search.Result[T], error),
) (search.Result[T], error) {
	if limit <= 0 {
		limit = legacyListDefaultLimit
	}
	pagination := search.Pagination{
		Limit:  int32(min(limit, legacyListPageSize)),
		Offset: int32(min(max(offset, 0), math.MaxInt32)),
	}

	var page search.Result[T]
	for remaining := limit; ; {
		result, err := fetch(pagination)
		if err != nil {
			return search.Result[T]{}, err
		}
		if page.Items == nil {
			page = result
		} else {
			page.Items = append(page.Items, result.Items...)
			page.NextCursor = result.NextCursor
		}
		remaining -= int64(len(result.Items))
		if remaining <= 0 || result.NextCursor == "" {
			break
		}
		pagination = search.Pagination{Limit: int32(min(remaining, legacyListPageSize)), Cursor: result.NextCursor}
	}
	page.Count = len(page.Items)
	return page, nil
}
