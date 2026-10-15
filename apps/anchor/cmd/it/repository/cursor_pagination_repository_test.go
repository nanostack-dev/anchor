package repository_test

import (
	"context"
	"testing"
	"time"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/product/apikey"
	resourcepermission "anchor/internal/domain/product/resource_permission"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	cursorFixtureRows = 7
	cursorPageLimit   = 3
)

var cursorTiedAt = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

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

func TestOrganizationRepositorySearchByProductIDWalksTiedRowsByCursor(t *testing.T) {
	productID := tenantProductChain(t)
	for range cursorFixtureRows {
		createOrganization(t, productID)
	}
	_, err := repoCtx.DB.Exec(
		`UPDATE organizations SET created_at = $1 WHERE product_id = $2`, cursorTiedAt, productID,
	)
	require.NoError(t, err)

	type request = search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]
	sorted := []search.Sort[organization.SortFieldProductOrganization]{
		{Field: organization.SortFieldProductOrganizationCreatedAt, Direction: search.SortAscending},
	}

	got, pages := walkByCursor(t,
		func(pagination search.Pagination) (search.Result[organization.Organization], error) {
			return repoCtx.OrganizationRepository.SearchByProductID(
				context.Background(), productID, request{Sort: sorted, Pagination: pagination},
			)
		},
		func(item organization.Organization) string { return item.ID },
	)

	want := queryIDs(t, `SELECT id FROM organizations WHERE product_id = $1 ORDER BY created_at ASC, id ASC`, productID)
	assert.Equal(t, want, got, "every row once, ties broken by id")
	assert.Equal(t, 3, pages)

	t.Run("a cursor with an offset is refused", func(t *testing.T) {
		first, firstErr := repoCtx.OrganizationRepository.SearchByProductID(
			context.Background(),
			productID,
			request{Sort: sorted, Pagination: search.Pagination{Limit: cursorPageLimit}},
		)
		require.NoError(t, firstErr)
		require.NotEmpty(t, first.NextCursor)

		_, refused := repoCtx.OrganizationRepository.SearchByProductID(
			context.Background(), productID,
			request{Sort: sorted, Pagination: search.Pagination{Limit: 1, Offset: 1, Cursor: first.NextCursor}},
		)
		assert.ErrorIs(t, refused, search.ErrCursorWithOffset)
	})

	t.Run("a cursor issued for another sort is refused", func(t *testing.T) {
		first, firstErr := repoCtx.OrganizationRepository.SearchByProductID(
			context.Background(),
			productID,
			request{Sort: sorted, Pagination: search.Pagination{Limit: cursorPageLimit}},
		)
		require.NoError(t, firstErr)
		require.NotEmpty(t, first.NextCursor)

		byName := []search.Sort[organization.SortFieldProductOrganization]{
			{Field: organization.SortFieldProductOrganizationName, Direction: search.SortAscending},
		}
		_, refused := repoCtx.OrganizationRepository.SearchByProductID(
			context.Background(), productID,
			request{Sort: byName, Pagination: search.Pagination{Limit: cursorPageLimit, Cursor: first.NextCursor}},
		)
		assert.ErrorIs(t, refused, search.ErrInvalidCursor)
	})

	t.Run("a malformed cursor is refused", func(t *testing.T) {
		_, refused := repoCtx.OrganizationRepository.SearchByProductID(
			context.Background(), productID,
			request{Sort: sorted, Pagination: search.Pagination{Limit: cursorPageLimit, Cursor: "not-a-cursor"}},
		)
		assert.ErrorIs(t, refused, search.ErrInvalidCursor)
	})
}

func TestProductResourcePermissionRepositorySearchByProductWalksCompositeKeyByCursor(t *testing.T) {
	productID := tenantProductChain(t)
	for _, name := range []string{"a:read", "b:read", "c:read", "d:read", "e:read", "f:read", "g:read"} {
		createResourcePermission(t, productID, name)
	}
	_, err := repoCtx.DB.Exec(
		`UPDATE product_resource_permissions SET created_at = $1 WHERE product_id = $2`, cursorTiedAt, productID,
	)
	require.NoError(t, err)

	got, pages := walkByCursor(t,
		func(pagination search.Pagination) (search.Result[resourcepermission.ProductResourcePermission], error) {
			return repoCtx.ProductResourcePermissionRepository.SearchByProduct(
				context.Background(), productID,
				search.Request[
					resourcepermission.SearchProductResourcePermissionFilter,
					resourcepermission.SortFieldProductResourcePermission,
				]{Pagination: pagination},
			)
		},
		func(item resourcepermission.ProductResourcePermission) string { return item.Name },
	)

	// The primary key is (product_id, name) and the product is fixed by the
	// scope, so the name is the tiebreak.
	assert.Equal(t, []string{"a:read", "b:read", "c:read", "d:read", "e:read", "f:read", "g:read"}, got)
	assert.Equal(t, 3, pages)
}

func TestProductAPIKeyRepositorySearchByProductIDWalksNullableKeyByCursor(t *testing.T) {
	productID := tenantProductChain(t)
	usedAt := cursorTiedAt
	for i := range cursorFixtureRows {
		createProductAPIKey(t, productID, func(key *apikey.ProductAPIKey) {
			// Four keys were used at the same instant and three never were: ties on
			// both sides of the NULL boundary.
			if i < 4 {
				key.LastUsedAt = &usedAt
			}
		})
	}

	for _, direction := range []search.SortDirection{search.SortAscending, search.SortDescending} {
		t.Run(string(direction), func(t *testing.T) {
			sorted := []search.Sort[apikey.SortFieldProductAPIKey]{
				{Field: apikey.SortFieldProductAPIKeyLastUsed, Direction: direction},
			}
			got, _ := walkByCursor(t,
				func(pagination search.Pagination) (search.Result[apikey.ProductAPIKey], error) {
					return repoCtx.ProductAPIKeyRepository.SearchByProductID(
						context.Background(),
						apikey.SearchProductAPIKeysInput{
							ProductID: productID,
							Request: search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]{
								Sort: sorted, Pagination: pagination,
							},
						},
					)
				},
				func(item apikey.ProductAPIKey) string { return item.ID },
			)

			dir := string(direction)
			want := queryIDs(
				t,
				`SELECT id FROM product_api_keys WHERE product_id = $1 ORDER BY last_used_at `+dir+`, id `+dir,
				productID,
			)
			assert.Equal(t, want, got, "NULLs sort last ascending and first descending, and the cursor follows")
		})
	}
}

func TestOrganizationMembershipRepositorySearchByOrgIDWalksByCursor(t *testing.T) {
	productID, organizationID := tenantProductOrgChain(t)
	for range cursorFixtureRows {
		createOrgMembership(t, productID, organizationID)
	}
	_, err := repoCtx.DB.Exec(
		`UPDATE organization_memberships SET created_at = $1 WHERE organization_id = $2`, cursorTiedAt, organizationID,
	)
	require.NoError(t, err)

	// No sort at all: the members search once had no ORDER BY in this case. The
	// keyset ends in the member id, so the pages are still total.
	got, pages := walkByCursor(t,
		func(pagination search.Pagination) (search.Result[organization.Membership], error) {
			return repoCtx.OrganizationMembershipRepository.SearchByOrgID(
				context.Background(), productID, organizationID,
				search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{Pagination: pagination},
			)
		},
		func(item organization.Membership) string { return item.ProductUserID },
	)

	want := queryIDs(t,
		`SELECT product_user_id FROM organization_memberships WHERE organization_id = $1 ORDER BY product_user_id ASC`,
		organizationID)
	assert.Equal(t, want, got)
	assert.Equal(t, 3, pages)
}
