package api

import (
	"context"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/permission"
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
	searchRequest := mapToSearchProductPermissionInput(request.Body)

	input := permission.SearchProductPermissionInput{
		ProductID: request.ProductId,
		Request:   searchRequest,
	}

	result, nanostackErr := s.PermissionService.SearchByProductID(ctx, input)
	if nanostackErr != nil {
		return nil, nanostackErr
	}

	response := ProductPermissionListResponse{
		Count: result.Count,
		Items: functional.Slice(result.Items).Map(mapProductPermissionToResponse),
		Total: result.Total,
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
	if perm == nil {
		return GetProductPermission404JSONResponse{NotFoundJSONResponse(
			notFoundBody("PRODUCT_PERMISSION_NOT_FOUND", "Product Permission does not exist."),
		)}, nil
	}
	response := mapProductPermissionToResponse(*perm)
	return GetProductPermission200JSONResponse(response), nil
}

func mapToSearchProductPermissionInput(
	searchReqBody *SearchProductPermissionsJSONRequestBody,
) search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission] {
	filter := functional.FromPtr(searchReqBody.Filter).
		Map(func(f ProductPermissionFilter) permission.SearchProductPermissionFilter {
			return permission.SearchProductPermissionFilter{
				Names: lowerPermissionNames(f.Names),
			}
		}).
		ToPtr()
	var req search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]
	return req.WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(
		searchReqBody.Pagination,
	)
}

func lowerPermissionNames(names []string) []string {
	lowered := functional.Slice(names).Map(strings.ToLower).ToSlice()
	if lowered == nil {
		lowered = []string{}
	}
	return lowered
}
