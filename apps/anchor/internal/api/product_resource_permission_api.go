package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/repository"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
)

func mapProductResourcePermissionToResponse(
	perm resourcepermission.ProductResourcePermission,
) ProductResourcePermissionResponse {
	return ProductResourcePermissionResponse{
		ProductId:     perm.ProductID,
		Name:          perm.Name,
		Description:   perm.Description,
		ScopeModifier: perm.ScopeModifier,
		ResourceType:  perm.ResourceType,
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
		ResourceType:  req.ResourceType,
	}
}

func mapToUpdateProductResourcePermissionInput(
	productID, name string, req *UpdateProductResourcePermissionRequest,
) resourcepermission.UpdateProductResourcePermissionInput {
	return resourcepermission.UpdateProductResourcePermissionInput{
		ProductID:    productID,
		Name:         name,
		Description:  req.Description,
		ResourceType: req.ResourceType,
	}
}

// productResourcePermissionSearch builds the search from the request body. The
// product scope comes from the route, which the authorization middleware
// checked against the principal; the body never sets it. The resource_types
// slugs go to the service, which resolves them to type IDs.
func productResourcePermissionSearch(
	productID string, body *ProductResourcePermissionSearchRequest,
) (repository.ProductResourcePermissionSearch, []string) {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	q := repository.NewProductResourcePermissionSearch(productID)
	q = searchexec.Some(q, q.Name().In, f.Names)
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.ProductResourcePermissionSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination), f.ResourceTypes
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
	if err := validateProductResourcePermissionsSearch(request.ProductId, request.Body); err != nil {
		return nil, err
	}
	q, resourceTypes := productResourcePermissionSearch(request.ProductId, request.Body)

	result, err := s.ResourcePermissionService.SearchByProduct(ctx, q, resourceTypes)
	if err != nil {
		return nil, err
	}

	response := ProductResourcePermissionListResponse{
		Count:      result.Count,
		Items:      functional.Slice(result.Items).Map(mapProductResourcePermissionToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
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

	response := mapProductResourcePermissionToResponse(resourcePermission)
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
