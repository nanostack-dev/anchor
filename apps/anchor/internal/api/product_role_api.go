package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/product/role"
)

func mapProductRoleToResponse(productRole role.ProductRole) ProductRoleResponse {
	return ProductRoleResponse{
		Id:          productRole.ID,
		ProductId:   productRole.ProductID,
		Name:        productRole.Name,
		Description: &productRole.Description,
		CreatedAt:   productRole.CreatedAt,
		UpdatedAt:   productRole.UpdatedAt,
		Permissions: functional.Slice(productRole.Permissions).Map(
			func(perm role.ProductRolePermission) ProductRolePermissionResponse {
				return ProductRolePermissionResponse{
					PermissionName: perm.PermissionName,
					ProductId:      perm.ProductID,
					ProductRoleId:  perm.ProductRoleID,
				}
			}),
	}
}

func (s *AnchorAPI) CreateProductRole(
	ctx context.Context, request CreateProductRoleRequestObject,
) (CreateProductRoleResponseObject, error) {
	input := role.CreateProductRoleInput{
		ProductID:   request.ProductId,
		Name:        request.Body.Name,
		Permissions: request.Body.Permissions,
	}
	if request.Body.Description != nil {
		input.Description = *request.Body.Description
	}

	productRole, err := s.ProductRoleService.CreateProductRole(ctx, input)
	if err != nil {
		return nil, err
	}

	response := mapProductRoleToResponse(productRole)
	return CreateProductRole201JSONResponse(response), nil
}

func (s *AnchorAPI) SearchProductRoles(
	ctx context.Context, request SearchProductRolesRequestObject,
) (SearchProductRolesResponseObject, error) {
	searchRequest := mapToSearchProductRoleInput(request.Body)

	input := role.SearchProductRolesInput{
		ProductID: request.ProductId,
		Request:   searchRequest,
	}

	result, err := s.ProductRoleService.SearchProductRoles(ctx, input)
	if err != nil {
		return nil, err
	}

	response := ProductRoleListResponse{
		Count: result.Count,
		Items: functional.Slice(result.Items).Map(mapProductRoleToResponse),
		Total: result.Total,
	}

	return SearchProductRoles200JSONResponse(response), nil
}

func (s *AnchorAPI) GetProductRole(
	ctx context.Context, request GetProductRoleRequestObject,
) (GetProductRoleResponseObject, error) {
	input := role.GetProductRoleInput{
		ProductID: request.ProductId,
		ID:        request.RoleId,
	}

	productRole, err := s.ProductRoleService.GetProductRole(ctx, input)
	if err != nil {
		return nil, err
	}
	if productRole == nil {
		return GetProductRole404JSONResponse{NotFoundJSONResponse(
			notFoundBody("PRODUCT_ROLE_NOT_FOUND", "Product Role does not exist."),
		)}, nil
	}

	response := mapProductRoleToResponse(*productRole)
	return GetProductRole200JSONResponse(response), nil
}

func (s *AnchorAPI) UpdateProductRole(
	ctx context.Context, request UpdateProductRoleRequestObject,
) (UpdateProductRoleResponseObject, error) {
	input := role.UpdateProductRoleInput{
		ProductID: request.ProductId,
		ID:        request.RoleId,
		Name:      &request.Body.Name,
	}
	if request.Body.Description != nil {
		input.Description = request.Body.Description
	}

	input.Permissions = s.mapToProductRolePermissionSlice(
		request.ProductId, request.RoleId,
		request.Body.Permissions,
	)
	updated, err := s.ProductRoleService.UpdateProductRole(ctx, input)
	if err != nil {
		return nil, err
	}

	response := mapProductRoleToResponse(updated)
	return UpdateProductRole200JSONResponse(response), nil
}

func (s *AnchorAPI) mapToProductRolePermissionSlice(
	productID, roleID string, permissionsStrings []string,
) []role.ProductRolePermission {
	if len(permissionsStrings) == 0 {
		return nil
	}
	return functional.Slice(permissionsStrings).Map(func(perm string) role.ProductRolePermission {
		return role.ProductRolePermission{
			ProductRoleID:  roleID,
			ProductID:      productID,
			PermissionName: perm,
		}
	})
}

func (s *AnchorAPI) DeleteProductRole(
	ctx context.Context, request DeleteProductRoleRequestObject,
) (DeleteProductRoleResponseObject, error) {
	input := role.DeleteProductRoleInput{
		ProductID: request.ProductId,
		ID:        request.RoleId,
	}

	err := s.ProductRoleService.DeleteProductRole(ctx, input)
	if err != nil {
		return nil, err
	}

	return DeleteProductRole204Response{}, nil
}

func (s *AnchorAPI) AssignPermissionToProductRole(
	ctx context.Context, request AssignPermissionToProductRoleRequestObject,
) (AssignPermissionToProductRoleResponseObject, error) {
	input := role.AssignPermissionToProductRoleInput{
		ProductID:      request.ProductId,
		ProductRoleID:  request.RoleId,
		PermissionName: request.Body.PermissionName,
	}

	_, err := s.ProductRoleService.AssignPermissionToProductRole(ctx, input)
	if err != nil {
		return nil, err
	}

	return AssignPermissionToProductRole204Response{}, nil
}

func (s *AnchorAPI) UnassignPermissionFromProductRole(
	ctx context.Context, request UnassignPermissionFromProductRoleRequestObject,
) (UnassignPermissionFromProductRoleResponseObject, error) {
	input := role.UnassignPermissionFromProductRoleInput{
		ProductID:      request.ProductId,
		ProductRoleID:  request.RoleId,
		PermissionName: request.PermissionId,
	}

	_, err := s.ProductRoleService.UnassignPermissionFromProductRole(ctx, input)
	if err != nil {
		return nil, err
	}

	return UnassignPermissionFromProductRole204Response{}, nil
}

func mapToSearchProductRoleInput(
	searchReqBody *SearchProductRolesJSONRequestBody,
) search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole] {
	filter := functional.FromPtr(searchReqBody.Filter).Map(func(f ProductRoleFilter) role.SearchProductRoleFilter {
		return role.SearchProductRoleFilter{
			ProductRoleIDs: f.Ids,
			Names:          f.Names,
		}
	}).ToPtr()
	var req search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]
	return req.WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(
		searchReqBody.Pagination,
	)
}
