package service_test

import (
	"context"
	"slices"
	"testing"

	resourcepermission "anchor/internal/domain/product/resource_permission"
	role "anchor/internal/domain/product/role"
	"anchor/internal/repository"
	"anchor/internal/service"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rolePermissionCatalog struct {
	repository.ProductResourcePermissionRepository
	productID string
	names     []string
}

func (c *rolePermissionCatalog) SearchByProduct(
	_ context.Context,
	productID string,
	_ search.Request[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission],
) (search.Result[resourcepermission.ProductResourcePermission], error) {
	c.productID = productID
	items := make([]resourcepermission.ProductResourcePermission, 0, len(c.names))
	for _, name := range c.names {
		items = append(items, resourcepermission.ProductResourcePermission{Name: name})
	}
	return search.Result[resourcepermission.ProductResourcePermission]{Items: items}, nil
}

func TestResolveRolePermissionsPreservesInput(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		requested []string
		expected  []string
		missing   bool
	}{
		{"case duplicate catalog uses last spelling", []string{"document:READ", "DOCUMENT:read"}, []string{"Document:Read", "Document:Read"}, false},
		{"missing name leaves valid input untouched", []string{"document:READ", "missing:Grant"}, nil, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			catalog := &rolePermissionCatalog{names: []string{"document:read", "Document:Read"}}
			input := make([]role.ProductRolePermission, 0, len(test.requested))
			for _, name := range test.requested {
				input = append(
					input,
					role.ProductRolePermission{
						ID:             "grant",
						ProductID:      "product",
						ProductRoleID:  "role",
						PermissionName: name,
					},
				)
			}
			original := slices.Clone(input)
			resolved, err := service.ResolveRolePermissions(t.Context(), "product", input, catalog)
			assert.Equal(t, original, input)
			assert.Equal(t, "product", catalog.productID)
			if test.missing {
				require.Error(t, err)
				assert.Nil(t, resolved)
				return
			}
			require.NoError(t, err)
			require.Len(t, resolved, len(test.expected))
			for i, name := range test.expected {
				expected := original[i]
				expected.PermissionName = name
				assert.Equal(t, expected, resolved[i])
			}
			resolved[0].PermissionName = "changed"
			assert.Equal(t, original, input)
		})
	}
}
