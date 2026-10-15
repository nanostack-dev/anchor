package ct_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	cursorPageSize     = 3
	cursorMinimumPages = 3
	// cursorDefaultPageSize is the page of a request that names no limit.
	cursorDefaultPageSize = 10
	invalidCursorCode     = "INVALID_CURSOR"
)

// searchCall posts one search body to one operation and returns the raw answer.
// The cursor tests drive every search through it, so one walker covers them all.
type searchCall func(ctx context.Context, body io.Reader) (status int, raw []byte, err error)

// cursorSort is one ordering a search is walked in: the sort_by it is asked for
// (empty means the documented default) and the SQL that lists the same ids.
type cursorSort struct {
	// name labels the subtest.
	name string
	// sortBy is sent as sort_by. Empty leaves the default in force.
	sortBy string
	// descending is sent as sort_direction when sortBy is set, and tells the
	// walker which way the SQL below runs.
	descending bool
	// omitDirection leaves sort_direction out, so the documented default
	// direction applies.
	omitDirection bool
	// order lists the ids a full walk must return, in order.
	order func(t *testing.T) []string
}

// cursorCase is one search under test.
type cursorCase struct {
	call searchCall
	// field is the sort_by that names the default sort. The cursor-for-another-sort
	// check sends it with the opposite direction.
	field string
	// idOf reads an item's identity; nil reads "id".
	idOf func(item map[string]any) string
	// sorts are walked one after the other. Each is walked over the same rows,
	// which the case has tied on its sort columns beforehand.
	sorts []cursorSort
}

type searchPage struct {
	ids        []string
	total      int64
	count      int
	nextCursor string
}

type searchPageRequest struct {
	limit      int
	offset     int
	cursor     string
	sortBy     string
	descending bool
	// noDirection omits sort_direction even when sortBy is set.
	noDirection bool
}

func (r searchPageRequest) body(t *testing.T) io.Reader {
	t.Helper()
	pagination := map[string]any{}
	if r.limit > 0 {
		pagination["limit"] = r.limit
	}
	if r.offset > 0 {
		pagination["offset"] = r.offset
	}
	if r.cursor != "" {
		pagination["cursor"] = r.cursor
	}
	body := map[string]any{"pagination": pagination}
	if r.sortBy != "" {
		body["sort_by"] = r.sortBy
	}
	if r.sortBy != "" && !r.noDirection {
		body["sort_direction"] = "ASC"
		if r.descending {
			body["sort_direction"] = "DESC"
		}
	}
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	return bytes.NewReader(encoded)
}

func (c cursorCase) fetch(t *testing.T, request searchPageRequest) searchPage {
	t.Helper()
	status, raw, err := c.call(context.Background(), request.body(t))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, string(raw))

	var decoded struct {
		Items      []map[string]any `json:"items"`
		Total      int64            `json:"total"`
		Count      int              `json:"count"`
		NextCursor string           `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded), string(raw))

	idOf := c.idOf
	if idOf == nil {
		idOf = func(item map[string]any) string { return fmt.Sprint(item["id"]) }
	}
	page := searchPage{total: decoded.Total, count: decoded.Count, nextCursor: decoded.NextCursor}
	for _, item := range decoded.Items {
		page.ids = append(page.ids, idOf(item))
	}
	return page
}

// rejected posts a body that must answer 400 INVALID_CURSOR.
func (c cursorCase) rejected(t *testing.T, request searchPageRequest) {
	t.Helper()
	status, raw, err := c.call(context.Background(), request.body(t))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, status, string(raw))

	var decoded struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded), string(raw))
	require.NotEmpty(t, decoded.Errors, string(raw))
	assert.Equal(t, invalidCursorCode, decoded.Errors[0].Code, string(raw))
}

type cursorWalk struct {
	ids   []string
	pages int
	total int64
}

// walk follows next_cursor from the first page to the last.
func (c cursorCase) walk(t *testing.T, sort cursorSort, limit int) cursorWalk {
	t.Helper()
	var walked cursorWalk
	request := searchPageRequest{
		limit: limit, sortBy: sort.sortBy, descending: sort.descending, noDirection: sort.omitDirection,
	}
	for {
		page := c.fetch(t, request)
		walked.pages++
		walked.ids = append(walked.ids, page.ids...)
		walked.total = page.total
		assert.Equal(t, len(page.ids), page.count, "count is the length of the page")
		if page.nextCursor == "" {
			return walked
		}
		assert.Len(t, page.ids, limit, "a page with a next_cursor is full")
		require.LessOrEqual(t, walked.pages, 1000, "a cursor walk must end")
		request.cursor = page.nextCursor
	}
}

func (c cursorCase) run(t *testing.T) {
	t.Helper()
	require.NotEmpty(t, c.sorts)
	require.NotEmpty(t, c.field)

	for _, sort := range c.sorts {
		t.Run("walks "+sort.name, func(t *testing.T) {
			expected := sort.order(t)
			require.GreaterOrEqual(t, len(expected), cursorPageSize*(cursorMinimumPages-1)+1,
				"the fixture must fill at least %d pages", cursorMinimumPages)
			require.Len(t, slices.Compact(slices.Clone(expected)), len(expected), "ids are unique")

			walked := c.walk(t, sort, cursorPageSize)
			assert.Equal(t, expected, walked.ids, "every row once, in the documented order")
			assert.GreaterOrEqual(t, walked.pages, cursorMinimumPages)
			assert.Equal(
				t,
				int64(len(expected)),
				walked.total,
				"total counts the filter, not the rows after the cursor",
			)

			all := c.fetch(t, searchPageRequest{
				limit: len(expected), sortBy: sort.sortBy, descending: sort.descending, noDirection: sort.omitDirection,
			})
			assert.Equal(t, expected, all.ids)
			assert.Empty(t, all.nextCursor, "a last page that fills the limit exactly has no next_cursor")

			offsetPage := c.fetch(t, searchPageRequest{
				limit: cursorPageSize, offset: cursorPageSize, sortBy: sort.sortBy, descending: sort.descending,
				noDirection: sort.omitDirection,
			})
			assert.Equal(
				t,
				expected[cursorPageSize:2*cursorPageSize],
				offsetPage.ids,
				"offset pages stay in the same order",
			)
			assert.NotEmpty(t, offsetPage.nextCursor, "an offset page hands over a cursor")
		})
	}

	first := c.sorts[0]
	t.Run("continues a cursor sent without a limit", func(t *testing.T) {
		expected := first.order(t)
		page := c.fetch(t, searchPageRequest{
			limit: cursorPageSize, sortBy: first.sortBy, descending: first.descending, noDirection: first.omitDirection,
		})
		require.NotEmpty(t, page.nextCursor)

		rest := c.fetch(t, searchPageRequest{
			cursor:      page.nextCursor,
			sortBy:      first.sortBy,
			descending:  first.descending,
			noDirection: first.omitDirection,
		})
		assert.Equal(t, expected[cursorPageSize:min(len(expected), cursorPageSize+cursorDefaultPageSize)], rest.ids,
			"the default page size applies")
	})

	t.Run("rejects a sort_by outside the contract", func(t *testing.T) {
		status, raw, err := c.call(context.Background(), strings.NewReader(`{"sort_by":"not_a_sortable_field"}`))
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, status, string(raw))
		assert.Contains(t, string(raw), "sort_by", "the error names the field")
		assert.Contains(t, string(raw), "VALIDATION_ERROR")
	})

	t.Run("rejects a malformed cursor", func(t *testing.T) {
		c.rejected(t, searchPageRequest{
			limit: cursorPageSize, cursor: "not-a-cursor", sortBy: first.sortBy, descending: first.descending,
			noDirection: first.omitDirection,
		})
	})

	t.Run("rejects a cursor with an offset", func(t *testing.T) {
		page := c.fetch(t, searchPageRequest{
			limit: cursorPageSize, sortBy: first.sortBy, descending: first.descending, noDirection: first.omitDirection,
		})
		require.NotEmpty(t, page.nextCursor)
		c.rejected(t, searchPageRequest{
			limit: cursorPageSize, offset: 1, cursor: page.nextCursor, sortBy: first.sortBy,
			descending: first.descending, noDirection: first.omitDirection,
		})
	})

	t.Run("rejects a cursor issued for another sort", func(t *testing.T) {
		page := c.fetch(t, searchPageRequest{
			limit: cursorPageSize, sortBy: first.sortBy, descending: first.descending, noDirection: first.omitDirection,
		})
		require.NotEmpty(t, page.nextCursor)
		c.rejected(t, searchPageRequest{
			limit: cursorPageSize, cursor: page.nextCursor, sortBy: c.field, descending: !first.descending,
		})
	})
}

// idsFromSQL runs a query that lists ids and returns them in query order.
func idsFromSQL(t *testing.T, query string, args ...any) []string {
	t.Helper()
	rows, err := testDB.QueryContext(t.Context(), query, args...)
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

func execSQL(t *testing.T, query string, args ...any) {
	t.Helper()
	_, err := testDB.ExecContext(t.Context(), query, args...)
	require.NoError(t, err)
}

// direction renders ASC or DESC for an ORDER BY.
func direction(descending bool) string {
	if descending {
		return "DESC"
	}
	return "ASC"
}

// tiedAt is the instant every row of a cursor fixture shares, so the sort column
// ties and only the primary key tells the rows apart.
func tiedAt() time.Time {
	return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
}
