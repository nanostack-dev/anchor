package searchx_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/searchx"
)

type sortField string

func TestIsCursorError(t *testing.T) {
	t.Parallel()

	assert.True(t, searchx.IsCursorError(search.ErrInvalidCursor))
	assert.True(t, searchx.IsCursorError(search.ErrCursorWithOffset))
	assert.True(t, searchx.IsCursorError(fmt.Errorf("repository: %w", search.ErrInvalidCursor)))
	assert.False(t, searchx.IsCursorError(errors.New("connection refused")))
	assert.False(t, searchx.IsCursorError(nil))
}

func TestSort(t *testing.T) {
	t.Parallel()

	name := sortField("name")
	desc := search.SortDescending
	asc := search.SortAscending

	t.Run("an omitted sort_by takes the default field and direction", func(t *testing.T) {
		t.Parallel()
		field, direction := searchx.Sort(nil, nil, sortField("created_at"), desc)
		require.NotNil(t, field)
		require.NotNil(t, direction)
		assert.Equal(t, sortField("created_at"), *field)
		assert.Equal(t, desc, *direction)
	})

	t.Run("an omitted sort_by keeps the direction the client named", func(t *testing.T) {
		t.Parallel()
		field, direction := searchx.Sort(nil, &asc, sortField("created_at"), desc)
		assert.Equal(t, sortField("created_at"), *field)
		assert.Equal(t, asc, *direction)
	})

	t.Run("a named sort_by without a direction stays ascending", func(t *testing.T) {
		t.Parallel()
		field, direction := searchx.Sort(&name, nil, sortField("created_at"), desc)
		assert.Equal(t, name, *field)
		assert.Nil(t, direction, "search.Request.WithSort sorts it ascending")
	})
}
