package api

import (
	"context"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/permission"
	"anchor/internal/repository"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
)

func mapProductPermissionToResponse(perm permission.ProductPermission) ProductPermissionResponse {
	return ProductPermissionResponse{
		ProductId:   perm.ProductID,
		Name:        perm.Name,
		Description: perm.Description,
		CreatedAt:   perm.CreatedAt,
		UpdatedAt:   perm.UpdatedAt,
	}
}

func (s *AnchorAPI) SearchProductPermissions(
	ctx context.Context, request SearchProductPermissionsRequestObject,
) (SearchProductPermissionsResponseObject, error) {
	q, err := productPermissionSearch(request.ProductId, request.Body)
	if err != nil {
		return nil, err
	}

	result, nanostackErr := s.PermissionService.SearchByProductID(ctx, q)
	if nanostackErr != nil {
		return nil, nanostackErr
	}

	response := ProductPermissionListResponse{
		Count:      result.Count,
		Items:      functional.Slice(result.Items).Map(mapProductPermissionToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
	}

	return SearchProductPermissions200JSONResponse(response), nil
}

func (s *AnchorAPI) GetProductPermission(
	ctx context.Context, request GetProductPermissionRequestObject,
) (GetProductPermissionResponseObject, error) {
	input := permission.FindProductPermissionInput{
		ProductID: request.ProductId,
		Name:      strings.ToLower(request.PermissionId),
	}

	perm, nanostackErr := s.PermissionService.FindByProductAndPermissionName(ctx, input)
	if nanostackErr != nil {
		return nil, nanostackErr
	}
	response := mapProductPermissionToResponse(perm)
	return GetProductPermission200JSONResponse(response), nil
}

// productPermissionSearch builds the search from the request body. The product
// scope comes from the route, which the authorization middleware checked
// against the principal; the body never sets it.
func productPermissionSearch(
	productID string, body *SearchProductPermissionsJSONRequestBody,
) (repository.ProductPermissionSearch, error) {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	names := lowerPermissionNames(f.Names)
	if err := validate.ValidateStruct(permission.SearchProductPermissionNames{Names: names}); err != nil {
		return repository.ProductPermissionSearch{}, err
	}
	q := repository.NewProductPermissionSearch(productID)
	q = searchexec.Some(q, q.Name().In, names)
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.ProductPermissionSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination), nil
}

func lowerPermissionNames(names []string) []string {
	lowered := functional.Slice(names).Map(strings.ToLower).ToSlice()
	if lowered == nil {
		lowered = []string{}
	}
	return lowered
}
