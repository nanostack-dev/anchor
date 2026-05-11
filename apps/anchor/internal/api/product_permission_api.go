package api

import (
	"context"

	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

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
		Items: toolkit.TransformSlice(result.Items, mapProductPermissionToResponse),
		Total: result.Total,
	}

	return SearchProductPermissions200JSONResponse(response), nil
}

func (s *AnchorAPI) GetProductPermission(
	ctx context.Context, request GetProductPermissionRequestObject,
) (GetProductPermissionResponseObject, error) {
	input := permission.FindProductPermissionInput{
		ProductID: request.ProductId,
		Name:      request.PermissionId,
	}

	perm, nanostackErr := s.PermissionService.FindByProductAndPermissionName(ctx, input)
	if nanostackErr != nil {
		return nil, nanostackErr
	}
	if perm == nil {
		return GetProductPermission404Response{}, nil
	}
	response := mapProductPermissionToResponse(*perm)
	return GetProductPermission200JSONResponse(response), nil
}

func mapToSearchProductPermissionInput(
	searchReqBody *SearchProductPermissionsJSONRequestBody,
) search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission] {
	var filter *permission.SearchProductPermissionFilter
	if searchReqBody.Filter != nil {
		filter = &permission.SearchProductPermissionFilter{
			Names: searchReqBody.Filter.Names,
		}
	}
	var req search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission]
	return req.WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(
		searchReqBody.Pagination,
	)
}
