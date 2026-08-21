package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	"anchor/internal/domain/product/apikey"
)

func mapProductAPIKeyToResponse(productAPIKey apikey.ProductAPIKey) ProductAPIKeyResponse {
	return ProductAPIKeyResponse{
		Id:              productAPIKey.ID,
		ProductId:       productAPIKey.ProductID,
		Name:            productAPIKey.Name,
		Description:     productAPIKey.Description,
		Mutable:         productAPIKey.Mutable,
		ObfuscatedValue: productAPIKey.ObfuscatedValue,
		LastUsedAt:      productAPIKey.LastUsedAt,
		Status:          productAPIKey.Status,
		CreatedAt:       productAPIKey.CreatedAt,
		UpdatedAt:       productAPIKey.UpdatedAt,
		Permissions: slicex.Map(
			productAPIKey.Permissions,
			func(perm apikey.ProductAPIKeyPermission) ProductAPIKeyPermissionResponse {
				return ProductAPIKeyPermissionResponse{
					ProductId:       perm.ProductID,
					ProductApiKeyId: perm.APIKeyID,
					PermissionName:  perm.PermissionName,
				}
			},
		),
	}
}

func mapCreatedProductAPIKeyToResponse(
	productAPIKey apikey.ProductAPIKey, clearValue string,
) CreatedProductAPIKeyResponse {
	baseResponse := mapProductAPIKeyToResponse(productAPIKey)
	return CreatedProductAPIKeyResponse{
		Id:              baseResponse.Id,
		ProductId:       baseResponse.ProductId,
		Name:            baseResponse.Name,
		Description:     baseResponse.Description,
		Mutable:         baseResponse.Mutable,
		ObfuscatedValue: baseResponse.ObfuscatedValue,
		LastUsedAt:      baseResponse.LastUsedAt,
		Status:          baseResponse.Status,
		CreatedAt:       baseResponse.CreatedAt,
		UpdatedAt:       baseResponse.UpdatedAt,
		Permissions:     baseResponse.Permissions,
		Value:           clearValue,
	}
}

func (s *AnchorAPI) CreateProductAPIKey(
	ctx context.Context, request CreateProductAPIKeyRequestObject,
) (CreateProductAPIKeyResponseObject, error) {
	input := apikey.CreateProductAPIKeyInput{
		ProductID:   request.ProductId,
		Name:        request.Body.Name,
		Description: request.Body.Description,
		Mutable:     request.Body.Mutable != nil && *request.Body.Mutable,
		Permissions: request.Body.Permissions,
	}

	productAPIKey, clearValue, err := s.ProductAPIKeyService.Create(ctx, input)
	if err != nil {
		return nil, err
	}

	response := mapCreatedProductAPIKeyToResponse(productAPIKey, clearValue)
	return CreateProductAPIKey201JSONResponse(response), nil
}

func (s *AnchorAPI) SearchProductAPIKeys(
	ctx context.Context, request SearchProductAPIKeysRequestObject,
) (SearchProductAPIKeysResponseObject, error) {
	searchRequest := mapToSearchProductAPIKeyInput(request.Body)

	input := apikey.SearchProductAPIKeysInput{
		ProductID: request.ProductId,
		Request:   searchRequest,
	}

	result, err := s.ProductAPIKeyService.Search(ctx, input)
	if err != nil {
		return nil, err
	}

	response := ProductAPIKeyListResponse{
		Count: result.Count,
		Items: slicex.Map(result.Items, mapProductAPIKeyToResponse),
		Total: result.Total,
	}

	return SearchProductAPIKeys200JSONResponse(response), nil
}

func (s *AnchorAPI) GetProductAPIKey(
	ctx context.Context, request GetProductAPIKeyRequestObject,
) (GetProductAPIKeyResponseObject, error) {
	input := apikey.GetProductAPIKeyInput{
		ProductID: request.ProductId,
		ID:        request.ApiKeyId,
	}

	productAPIKey, err := s.ProductAPIKeyService.GetByID(ctx, input)
	if err != nil {
		return nil, err
	}
	if productAPIKey == nil {
		return GetProductAPIKey404JSONResponse{NotFoundJSONResponse(
			notFoundBody("PRODUCT_A_P_I_KEY_NOT_FOUND", "Product A P I Key does not exist."),
		)}, nil
	}

	response := mapProductAPIKeyToResponse(*productAPIKey)
	return GetProductAPIKey200JSONResponse(response), nil
}

func (s *AnchorAPI) UpdateProductAPIKey(
	ctx context.Context, request UpdateProductAPIKeyRequestObject,
) (UpdateProductAPIKeyResponseObject, error) {
	input := apikey.UpdateProductAPIKeyInput{
		ProductID:   request.ProductId,
		ID:          request.ApiKeyId,
		Name:        &request.Body.Name,
		Description: request.Body.Description,
		Permissions: request.Body.Permissions,
	}

	updated, err := s.ProductAPIKeyService.Update(ctx, input)
	if err != nil {
		return nil, err
	}

	response := mapProductAPIKeyToResponse(updated)
	return UpdateProductAPIKey200JSONResponse(response), nil
}

func (s *AnchorAPI) DeleteProductAPIKey(
	ctx context.Context, request DeleteProductAPIKeyRequestObject,
) (DeleteProductAPIKeyResponseObject, error) {
	input := apikey.DeleteProductAPIKeyInput{
		ProductID: request.ProductId,
		ID:        request.ApiKeyId,
	}

	err := s.ProductAPIKeyService.Delete(ctx, input)
	if err != nil {
		return nil, err
	}

	return DeleteProductAPIKey204Response{}, nil
}

func mapToSearchProductAPIKeyInput(
	searchReqBody *SearchProductAPIKeysJSONRequestBody,
) search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey] {
	var filter *apikey.SearchProductAPIKeyFilter
	if searchReqBody.Filter != nil {
		filter = &apikey.SearchProductAPIKeyFilter{
			ProductAPIKeyIDs: searchReqBody.Filter.Ids,
			Names:            searchReqBody.Filter.Names,
		}
		if searchReqBody.Filter.Status != nil {
			filter.Status = slicex.Map(
				*searchReqBody.Filter.Status, func(s ProductAPIKeyStatus) string {
					return string(s)
				},
			)
		}
	}
	var req search.Request[apikey.SearchProductAPIKeyFilter, apikey.SortFieldProductAPIKey]
	return req.WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(
		searchReqBody.Pagination,
	)
}
