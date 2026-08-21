package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	resourcepermission "anchor/internal/domain/product/resource_permission"
)

func mapProductResourcePermissionToResponse(
	perm resourcepermission.ProductResourcePermission,
) ProductResourcePermissionResponse {
	return ProductResourcePermissionResponse{
		ProductId:     perm.ProductID,
		Name:          perm.Name,
		Description:   perm.Description,
		ScopeModifier: perm.ScopeModifier,
		CreatedAt:     perm.CreatedAt,
		UpdatedAt:     perm.UpdatedAt,
	}
}

func mapToCreateProductResourcePermissionInput(
	productID string, req *CreateProductResourcePermissionRequest,
) resourcepermission.CreateProductResourcePermissionInput {
	return resourcepermission.CreateProductResourcePermissionInput{
		ProductID:     productID,
		Name:          req.Name,
		Description:   req.Description,
		ScopeModifier: req.ScopeModifier,
	}
}

func mapToUpdateProductResourcePermissionInput(
	productID, name string, req *UpdateProductResourcePermissionRequest,
) resourcepermission.UpdateProductResourcePermissionInput {
	return resourcepermission.UpdateProductResourcePermissionInput{
		ProductID:   productID,
		Name:        name,
		Description: req.Description,
	}
}

func mapToSearchProductResourcePermissionInput(
	req *ProductResourcePermissionSearchRequest,
) search.Request[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission] {
	var filter *resourcepermission.SearchProductResourcePermissionFilter
	if req.Filter != nil {
		filter = &resourcepermission.SearchProductResourcePermissionFilter{
			Names: req.Filter.Names,
		}
	}
	var searchReq search.Request[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission]
	return searchReq.
		WithFilter(filter).
		WithSort(req.SortBy, req.SortDirection).
		WithPagination(req.Pagination)
}

func (s *AnchorAPI) CreateProductResourcePermission(
	ctx context.Context, request CreateProductResourcePermissionRequestObject,
) (CreateProductResourcePermissionResponseObject, error) {
	input := mapToCreateProductResourcePermissionInput(request.ProductId, request.Body)

	resourcePermission, err := s.ResourcePermissionService.Create(ctx, input)
	if err != nil {
		return nil, err
	}

	response := mapProductResourcePermissionToResponse(resourcePermission)
	return CreateProductResourcePermission201JSONResponse(response), nil
}

func (s *AnchorAPI) SearchProductResourcePermissions(
	ctx context.Context, request SearchProductResourcePermissionsRequestObject,
) (SearchProductResourcePermissionsResponseObject, error) {
	searchRequest := mapToSearchProductResourcePermissionInput(request.Body)

	input := resourcepermission.SearchProductResourcePermissionInput{
		ProductID: request.ProductId,
		Request:   searchRequest,
	}

	result, err := s.ResourcePermissionService.SearchByProduct(ctx, input)
	if err != nil {
		return nil, err
	}

	response := ProductResourcePermissionListResponse{
		Count: result.Count,
		Items: slicex.Map(result.Items, mapProductResourcePermissionToResponse),
		Total: result.Total,
	}

	return SearchProductResourcePermissions200JSONResponse(response), nil
}

func (s *AnchorAPI) GetProductResourcePermission(
	ctx context.Context, request GetProductResourcePermissionRequestObject,
) (GetProductResourcePermissionResponseObject, error) {
	input := resourcepermission.GetProductResourcePermissionInput{
		ProductID:      request.ProductId,
		PermissionName: request.PermissionName,
	}

	resourcePermission, err := s.ResourcePermissionService.GetByID(ctx, input)
	if err != nil {
		return nil, err
	}

	if resourcePermission == nil {
		return GetProductResourcePermission404JSONResponse{NotFoundJSONResponse(
			notFoundBody("PRODUCT_RESOURCE_PERMISSION_NOT_FOUND", "Product Resource Permission does not exist."),
		)}, nil
	}

	response := mapProductResourcePermissionToResponse(*resourcePermission)
	return GetProductResourcePermission200JSONResponse(response), nil
}

func (s *AnchorAPI) UpdateProductResourcePermission(
	ctx context.Context, request UpdateProductResourcePermissionRequestObject,
) (UpdateProductResourcePermissionResponseObject, error) {
	input := mapToUpdateProductResourcePermissionInput(
		request.ProductId, request.PermissionName, request.Body,
	)

	resourcePermission, err := s.ResourcePermissionService.Update(ctx, input)
	if err != nil {
		return nil, err
	}

	response := mapProductResourcePermissionToResponse(resourcePermission)
	return UpdateProductResourcePermission200JSONResponse(response), nil
}

func (s *AnchorAPI) DeleteProductResourcePermission(
	ctx context.Context, request DeleteProductResourcePermissionRequestObject,
) (DeleteProductResourcePermissionResponseObject, error) {
	input := resourcepermission.DeleteProductResourcePermissionInput{
		ProductID: request.ProductId,
		Name:      request.PermissionName,
	}

	err := s.ResourcePermissionService.Delete(ctx, input)
	if err != nil {
		return nil, err
	}

	return DeleteProductResourcePermission204Response{}, nil
}
