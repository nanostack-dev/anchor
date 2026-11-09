package repository_test

import (
	"context"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	cursorFixtureRows = 7
	cursorPageLimit   = 3
)

// walkByCursor follows NextCursor from the first page to the last and returns
// every id once in the order the pages came in, with the number of pages.
func walkByCursor[T any](
	t *testing.T,
	fetch func(pagination search.Pagination) (search.Result[T], error),
	id func(T) string,
) ([]string, int) {
	t.Helper()

	var ids []string
	pagination := search.Pagination{Limit: cursorPageLimit}
	for pages := 1; ; pages++ {
		require.LessOrEqual(t, pages, 100, "a cursor walk must end")

		result, err := fetch(pagination)
		require.NoError(t, err)
		for _, item := range result.Items {
			ids = append(ids, id(item))
		}
		if result.NextCursor == "" {
			return ids, pages
		}
		assert.Len(t, result.Items, cursorPageLimit, "a page with a NextCursor is full")
		pagination = search.Pagination{Limit: cursorPageLimit, Cursor: result.NextCursor}
	}
}

func queryIDs(t *testing.T, query string, args ...any) []string {
	t.Helper()

	rows, err := repoCtx.DB.QueryContext(context.Background(), query, args...)
	require.NoError(t, err)
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	require.NoError(t, rows.Err())

	return ids
}
