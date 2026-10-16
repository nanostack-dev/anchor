package searchx_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/searchx"
)

type sortField string

func TestIsClientError(t *testing.T) {
	t.Parallel()

	assert.True(t, searchx.IsClientError(search.ErrInvalidCursor))
	assert.True(t, searchx.IsClientError(search.ErrCursorWithOffset))
	assert.True(t, searchx.IsClientError(search.ErrUnknownSort))
	assert.True(t, searchx.IsClientError(fmt.Errorf("%w: sparkle", search.ErrUnknownSort)))
	assert.True(t, searchx.IsClientError(fmt.Errorf("repository: %w", search.ErrInvalidCursor)))
	assert.False(t, searchx.IsClientError(errors.New("connection refused")))
	assert.False(t, searchx.IsClientError(nil))
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

type validatedInput struct {
	Request search.Request[struct{}, sortField] `validate:"required"`
}

func TestValidateRequest(t *testing.T) {
	t.Parallel()

	created := sortField("created_at")
	valid := func() validatedInput {
		request := search.NewRequest[struct{}, sortField]()
		return validatedInput{Request: request}
	}
	validate := func(in validatedInput) error {
		return searchx.ValidateRequest(in, in.Request, created)
	}

	t.Run("accepts a request that names a mapped sort", func(t *testing.T) {
		t.Parallel()
		in := valid()
		in.Request.Sort = []search.Sort[sortField]{{Field: created, Direction: search.SortAscending}}
		require.NoError(t, validate(in))
	})

	t.Run("refuses a sort the resource does not map as a validation error", func(t *testing.T) {
		t.Parallel()
		in := valid()
		in.Request.Sort = []search.Sort[sortField]{{Field: "nope", Direction: search.SortAscending}}
		err := validate(in)
		require.Error(t, err)
		require.NotErrorIs(t, err, search.ErrUnknownSort)
		assert.Contains(t, err.Error(), "sort_by")
	})

	t.Run("answers an oversized cursor with the cursor sentinel, not a tag error", func(t *testing.T) {
		t.Parallel()
		in := valid()
		in.Request.Pagination.Cursor = strings.Repeat("A", search.MaxCursorLength+1)
		assert.ErrorIs(t, validate(in), search.ErrInvalidCursor)
	})

	t.Run("refuses a cursor combined with an offset", func(t *testing.T) {
		t.Parallel()
		in := valid()
		in.Request.Pagination.Cursor = "abc"
		in.Request.Pagination.Offset = 1
		assert.ErrorIs(t, validate(in), search.ErrCursorWithOffset)
	})
}
