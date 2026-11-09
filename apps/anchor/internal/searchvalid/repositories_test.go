package searchvalid_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/product/role"
	"anchor/internal/domain/resource"
	"anchor/internal/repository"
	resourcerepository "anchor/internal/resource/repository"
)

// unknownSort asks for a sort field no search maps.
func unknownSort[F any, S ~string]() search.Request[F, S] {
	return search.Request[F, S]{
		Sort:       []search.Sort[S]{{Field: S("searchtest_unknown_field"), Direction: search.SortAscending}},
		Pagination: search.Pagination{Limit: 10},
	}
}

// Request validation refuses an unknown sort_by before a search reads anything,
// so the 400 an unknown sort earns over HTTP is a validation error. This test
// pins the line behind it: every hand-written search repository maps its sorts with
// transactor.StrictKeys, so a field that validation lets through but the
// repository does not map fails with search.ErrUnknownSort, which the error
// handler answers with 400 INVALID_SORT, instead of silently ordering by
// something else. The ordering is built before the first statement runs, so the
// repositories need no database here.
func TestSearchRepositoriesRefuseASortTheyDoNotMap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)
	tests := map[string]func() error{
		"searchResources": func() error {
			_, err := resourcerepository.NewRepository(nil).Search(
				ctx, "product", "organization", unknownSort[resource.Filter, resource.SortField]())
			return err
		},
		"searchProductRoles": func() error {
			_, err := repository.NewProductRoleRepository(nil, nil, logger).SearchByProductID(
				ctx, "product", unknownSort[role.SearchProductRoleFilter, role.SortFieldProductRole]())
			return err
		},
	}

	for name, run := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := run()
			require.Error(t, err)
			assert.ErrorIs(t, err, search.ErrUnknownSort)
		})
	}
}
