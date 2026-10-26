package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/organization"
	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/domain/permission"
	"anchor/internal/domain/product/apikey"
	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/repository"
	rtservice "anchor/internal/resourcetype/service"
	"anchor/internal/service"
)

// A search repository answers a request mistake with a sentinel: a cursor that
// does not continue the search, a cursor with an offset, or a sort field it does
// not map. The error handler turns each into a 400, but only if the service
// between them passes it through. These services used to collapse every
// repository failure into fault.ErrUnexpected, a 500, except the two cursor
// sentinels; the unknown sort fell through.
type failingSearches struct {
	err error
}

type failingPermissions struct {
	repository.ProductPermissionRepository
	failingSearches
}

func (f failingPermissions) SearchByProduct(
	context.Context, string,
	search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission],
) (search.Result[permission.ProductPermission], error) {
	return search.Result[permission.ProductPermission]{}, f.err
}

type failingResourcePermissions struct {
	repository.ProductResourcePermissionRepository
	failingSearches
}

func (f failingResourcePermissions) SearchByProduct(
	context.Context, string,
	search.Request[resourcepermission.SearchProductResourcePermissionFilter,
		resourcepermission.SortFieldProductResourcePermission],
) (search.Result[resourcepermission.ProductResourcePermission], error) {
	return search.Result[resourcepermission.ProductResourcePermission]{}, f.err
}

// organizationOnly is a product whose tree holds only the built-in root.
type organizationOnly struct {
	rtservice.Service
}

func (organizationOnly) Tree(_ context.Context, productID string) (resourcetype.Tree, error) {
	return resourcetype.NewTree([]resourcetype.ResourceType{resourcetype.NewOrganization(productID)}), nil
}

type failingRoles struct {
	repository.ProductRoleRepository
	failingSearches
}

func (f failingRoles) SearchByProductID(
	context.Context, string,
	search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole],
) (search.Result[role.ProductRole], error) {
	return search.Result[role.ProductRole]{}, f.err
}

type failingProductAPIKeys struct {
	repository.ProductAPIKeyRepository
	failingSearches
}

func (f failingProductAPIKeys) SearchByProductID(
	context.Context, apikey.SearchProductAPIKeysInput,
) (search.Result[apikey.ProductAPIKey], error) {
	return search.Result[apikey.ProductAPIKey]{}, f.err
}

type failingOrganizationAPIKeys struct {
	repository.OrganizationAPIKeyRepository
	failingSearches
}

func (f failingOrganizationAPIKeys) SearchByOrganizationID(
	context.Context, orgapikey.SearchOrganizationAPIKeysInput,
) (search.Result[orgapikey.OrganizationAPIKey], error) {
	return search.Result[orgapikey.OrganizationAPIKey]{}, f.err
}

type presentOrganization struct {
	repository.OrganizationRepository
}

func (presentOrganization) FindByID(
	context.Context, string, string,
) (functional.Option[organization.Organization], error) {
	return functional.Some(organization.Organization{}), nil
}

func TestSearchServicesPassRequestErrorsThrough(t *testing.T) {
	t.Parallel()

	logger := zerolog.Nop()
	ctx := context.Background()

	searches := map[string]func(err error) error{
		"searchProductPermissions": func(err error) error {
			svc := service.NewPermissionService(failingPermissions{failingSearches: failingSearches{err}}, logger)
			_, got := svc.SearchByProductID(ctx, permission.SearchProductPermissionInput{
				ProductID: "product",
				Request:   search.NewRequest[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission](),
			})
			return got
		},
		"searchProductResourcePermissions": func(err error) error {
			svc := service.NewResourcePermissionService(
				failingResourcePermissions{
					failingSearches: failingSearches{err},
				},
				nil,
				organizationOnly{},
				nil,
				nil,
				logger,
			)
			_, got := svc.SearchByProduct(ctx, resourcepermission.SearchProductResourcePermissionInput{
				ProductID: "product",
				Request: search.NewRequest[
					resourcepermission.SearchProductResourcePermissionFilter,
					resourcepermission.SortFieldProductResourcePermission,
				](),
			})
			return got
		},
		"searchProductRoles": func(err error) error {
			svc := service.NewProductRoleService(
				failingRoles{failingSearches: failingSearches{err}}, nil, nil, organizationOnly{}, nil, nil, logger,
			)
			_, got := svc.SearchProductRoles(ctx, role.SearchProductRolesInput{
				ProductID: "product",
				Request:   search.NewRequest[role.SearchProductRoleFilter, role.SortFieldProductRole](),
			})
			return got
		},
		"searchProductAPIKeys": func(err error) error {
			svc := service.NewProductAPIKeyService(
				nil, failingProductAPIKeys{failingSearches: failingSearches{err}}, nil, nil, logger,
			)
			_, got := svc.Search(ctx, apikey.SearchProductAPIKeysInput{
				ProductID: "product",
				Request:   search.NewRequest[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey](),
			})
			return got
		},
		"searchOrganizationAPIKeys": func(err error) error {
			svc := service.NewOrganizationAPIKeyService(
				nil, nil, failingOrganizationAPIKeys{failingSearches: failingSearches{err}},
				presentOrganization{}, nil, nil, nil, logger,
			)
			_, got := svc.Search(ctx, orgapikey.SearchOrganizationAPIKeysInput{
				ProductID:      "product",
				OrganizationID: "organization",
				Request: search.NewRequest[
					orgapikey.SearchOrganizationAPIKeyFilter, orgapikey.SortFieldOrganizationAPIKey,
				](),
			})
			return got
		},
	}

	sentinels := map[string]error{
		"an unknown sort":        fmt.Errorf("%w: searchtest_unknown_field", search.ErrUnknownSort),
		"an invalid cursor":      fmt.Errorf("%w: issued for a different sort", search.ErrInvalidCursor),
		"a cursor with a offset": search.ErrCursorWithOffset,
	}

	for name, run := range searches {
		for label, sentinel := range sentinels {
			t.Run(name+" passes "+label+" through", func(t *testing.T) {
				t.Parallel()
				got := run(sentinel)
				require.Error(t, got)
				assert.ErrorIs(t, got, sentinel, "the service must return the repository's error")
			})
		}

		t.Run(name+" still hides an unexpected failure", func(t *testing.T) {
			t.Parallel()
			got := run(errors.New("connection refused"))
			require.Error(t, got)
			assert.NotContains(t, got.Error(), "connection refused")
		})
	}
}
