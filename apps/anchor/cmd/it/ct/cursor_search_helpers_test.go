package ct_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"testing"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/testkit/searchtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// cursorPageSize is the page size of every walk: seven rows fill three pages
	// of three, the last one short.
	cursorPageSize = 3
	// cursorDefaultPageSize is the page of a request that names no limit.
	cursorDefaultPageSize = 10
	// sortRefusalCode is the code of the 400 an unknown sort_by answers. Every
	// search validates its input, sort included, before it reads anything, so an
	// unknown sort is a validation error. The repositories map their sorts
	// through transactor.StrictKeys as the second line of defence and the
	// services pass its search.ErrUnknownSort through; the unit tests of those
	// layers pin that, because validation makes it unreachable over HTTP.
	sortRefusalCode = "VALIDATION_ERROR"
)

// searchCall posts one search body to one operation and returns the raw answer.
// The cursor tests drive every search through it, so one suite shape covers them
// all.
type searchCall func(ctx context.Context, body io.Reader) (status int, raw []byte, err error)

// sortField is one sort_by a search accepts: the SQL that orders by the same
// column, and the NULL placement the key pins when it pins one.
type sortField struct {
	// name is the sort_by value.
	name string
	// expr is the SQL expression the field orders by.
	expr string
	// nulls is "NULLS LAST" or "NULLS FIRST" when the key pins the placement of
	// NULLs; empty keeps the Postgres default the framework keys follow.
	nulls string
}

// orderedRows describes the rows of a search and every ordering it accepts, so
// the expected order of each walk comes from SQL over the fixture, never from
// the search under test.
type orderedRows struct {
	// from is the FROM clause, with its joins and aliases.
	from string
	// where scopes the rows to the fixture.
	where string
	args  []any
	// idExpr is the SQL expression of the identity a page reports, and the
	// tiebreak of every ordering.
	idExpr string
	// defaultField is the sort_by that names the documented default order, and
	// defaultDescending the direction that order runs in.
	defaultField      string
	defaultDescending bool
	// fields lists every sort_by the search accepts, the default one included.
	fields []sortField
}

func (o orderedRows) field(t *testing.T, name string) sortField {
	t.Helper()
	for _, candidate := range o.fields {
		if candidate.name == name {
			return candidate
		}
	}
	require.FailNow(t, "the fixture does not list the sort field "+name)
	return sortField{}
}

// order lists the ids a full walk must return when ordered by field.
func (o orderedRows) order(t *testing.T, field sortField, descending bool) []string {
	t.Helper()
	dir := direction(descending)
	nulls := ""
	if field.nulls != "" {
		nulls = " " + field.nulls
	}
	query := fmt.Sprintf(
		"SELECT %[1]s FROM %[2]s WHERE %[3]s ORDER BY %[4]s %[5]s%[6]s, %[1]s %[5]s",
		o.idExpr, o.from, o.where, field.expr, dir, nulls,
	)
	return idsFromSQL(t, query, o.args...)
}

// cases walks the documented default order, the default field with its
// direction left out, and every sort field in both directions.
func (o orderedRows) cases(t *testing.T) []searchtest.Case {
	t.Helper()
	defaultField := o.field(t, o.defaultField)
	cases := []searchtest.Case{
		{
			Name: "default order",
			Body: map[string]any{},
			Want: o.order(t, defaultField, o.defaultDescending),
		},
		{
			Name: o.defaultField + " without a direction",
			Body: map[string]any{"sort_by": o.defaultField},
			Want: o.order(t, defaultField, false),
		},
	}
	for _, field := range o.fields {
		for _, descending := range []bool{false, true} {
			cases = append(cases, searchtest.Case{
				Name: field.name + " " + direction(descending),
				Body: sortBody(field.name, descending),
				Want: o.order(t, field, descending),
			})
		}
	}
	return cases
}

func sortBody(field string, descending bool) map[string]any {
	return map[string]any{"sort_by": field, "sort_direction": direction(descending)}
}

// cursorCase is one search under test.
type cursorCase struct {
	call searchCall
	// idField names the item property that carries the identity; empty means "id".
	idField string
	// decode replaces the default decoder for a response whose identity is not a
	// plain property of the item.
	decode searchtest.Decoder
	rows   orderedRows
	// noMatch is a body that matches no row, which makes the scope empty: the
	// search may answer it without reading a page, and must still validate the
	// whole input.
	noMatch map[string]any
}

func (c cursorCase) decoder() searchtest.Decoder {
	if c.decode != nil {
		return c.decode
	}
	idField := c.idField
	if idField == "" {
		idField = "id"
	}
	return searchtest.DecodeItems("items", idField)
}

func (c cursorCase) kitCall() searchtest.Call {
	return func(ctx context.Context, body map[string]any) (int, []byte, error) {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		return c.call(ctx, bytes.NewReader(encoded))
	}
}

// scopes lists the requests the refusals are sent in. A populated scope per
// sort field lets the forged cursors reach every key type of the search. The
// empty scopes cover the short-circuit a search may take when nothing matches.
func (c cursorCase) scopes() []searchtest.Scope {
	opposite := sortBody(c.rows.defaultField, !c.rows.defaultDescending)
	scopes := []searchtest.Scope{{Body: map[string]any{}, Incompatible: opposite}}
	for _, field := range c.rows.fields {
		scopes = append(scopes, searchtest.Scope{
			Body:         sortBody(field.name, false),
			Incompatible: sortBody(field.name, true),
		})
	}

	last := c.rows.fields[len(c.rows.fields)-1].name
	sorted := sortBody(last, true)
	scopes = append(scopes,
		searchtest.Scope{Body: c.noMatch, SameOrder: map[string]any{}, Incompatible: opposite},
		searchtest.Scope{
			Body:         withSort(c.noMatch, sorted),
			SameOrder:    sorted,
			Incompatible: sortBody(last, false),
		},
	)
	return scopes
}

func withSort(body map[string]any, sort map[string]any) map[string]any {
	merged := map[string]any{}
	maps.Copy(merged, body)
	maps.Copy(merged, sort)
	return merged
}

func (c cursorCase) suite(t *testing.T, name string) searchtest.Suite {
	t.Helper()
	require.NotEmpty(t, c.rows.fields)
	require.NotEmpty(t, c.noMatch)
	return searchtest.Suite{
		Name:   name,
		Call:   c.kitCall(),
		Decode: c.decoder(),
		Limit:  cursorPageSize,
		Cases:  c.rows.cases(t),
		Refusals: searchtest.Refusals{
			Scopes:    c.scopes(),
			SortField: true,
			SortCode:  sortRefusalCode,
		},
		Mutate: searchtest.FrameworkCursorMutations,
	}
}

// run applies the conformance kit, then the checks the kit does not make.
func (c cursorCase) run(t *testing.T, name string) {
	t.Helper()
	suite := c.suite(t, name)
	searchtest.Run(t, suite)

	fetch := func(t *testing.T, body map[string]any) searchtest.Page {
		t.Helper()
		status, raw, err := suite.Call(t.Context(), body)
		require.NoError(t, err)
		require.Equal(t, 200, status, string(raw))
		page, err := suite.Decode(raw)
		require.NoError(t, err, string(raw))
		return page
	}
	paginated := func(pagination map[string]any) map[string]any {
		return map[string]any{"pagination": pagination}
	}

	expected := suite.Cases[0].Want

	t.Run(name+"/a page that fills the limit exactly has no next_cursor", func(t *testing.T) {
		// A page filled exactly to its limit omits the cursor when no more rows remain.
		page := fetch(t, paginated(map[string]any{"limit": len(expected)}))
		assert.Equal(t, expected, page.IDs)
		assert.Empty(t, page.NextCursor)
	})

	t.Run(name+"/offset pages keep the order and hand over a cursor", func(t *testing.T) {
		// Offset pagination preserves the sort order and returns a continuation cursor.
		page := fetch(t, paginated(map[string]any{"limit": cursorPageSize, "offset": cursorPageSize}))
		assert.Equal(t, expected[cursorPageSize:2*cursorPageSize], page.IDs)
		assert.NotEmpty(t, page.NextCursor)
	})

	t.Run(name+"/a cursor sent without a limit gets the default page size", func(t *testing.T) {
		// A cursor request without a limit uses the default page size.
		first := fetch(t, paginated(map[string]any{"limit": cursorPageSize}))
		require.NotEmpty(t, first.NextCursor)
		rest := fetch(t, paginated(map[string]any{"cursor": first.NextCursor}))
		assert.Equal(t, expected[cursorPageSize:min(len(expected), cursorPageSize+cursorDefaultPageSize)], rest.IDs)
	})
}

// nestedIDDecoder reads a page whose items carry their identity under a child
// object, as the organizations of a product user do.
func nestedIDDecoder(child string) searchtest.Decoder {
	return func(response []byte) (searchtest.Page, error) {
		var body struct {
			Items      *[]map[string]json.RawMessage `json:"items"`
			Total      *int64                        `json:"total"`
			Count      *int                          `json:"count"`
			NextCursor *string                       `json:"next_cursor"`
		}
		if err := json.Unmarshal(response, &body); err != nil {
			return searchtest.Page{}, fmt.Errorf("not a JSON object: %w", err)
		}
		if body.Items == nil || body.Total == nil || body.Count == nil {
			return searchtest.Page{}, errors.New("missing items, total or count")
		}
		page := searchtest.Page{Total: *body.Total, Count: *body.Count}
		if body.NextCursor != nil {
			page.NextCursor = *body.NextCursor
		}
		for i, item := range *body.Items {
			var nested struct {
				ID *string `json:"id"`
			}
			raw, ok := item[child]
			if !ok {
				return searchtest.Page{}, fmt.Errorf("item %d: missing %q", i, child)
			}
			if err := json.Unmarshal(raw, &nested); err != nil || nested.ID == nil {
				return searchtest.Page{}, fmt.Errorf("item %d: %q has no id", i, child)
			}
			page.IDs = append(page.IDs, *nested.ID)
		}
		return page, nil
	}
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
