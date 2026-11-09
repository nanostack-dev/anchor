package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/product/apikey"
	"anchor/internal/repository"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
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
		Permissions: append([]ProductAPIKeyPermissionResponse{}, functional.Slice(
			productAPIKey.Permissions).Map(

			func(perm apikey.ProductAPIKeyPermission) ProductAPIKeyPermissionResponse {
				return ProductAPIKeyPermissionResponse{
					ProductId:       perm.ProductID,
					ProductApiKeyId: perm.APIKeyID,
					PermissionName:  perm.PermissionName,
				}
			})...),
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
	body := request.Body
	if body == nil {
		body = &SearchProductAPIKeysJSONRequestBody{}
	}
	if err := validateProductAPIKeysSearch(request.ProductId, body); err != nil {
		return nil, err
	}
	q := productAPIKeySearch(request.ProductId, body)

	result, err := s.ProductAPIKeyService.Search(ctx, q)
	if err != nil {
		return nil, err
	}

	response := ProductAPIKeyListResponse{
		Count:      result.Count,
		Items:      functional.Slice(result.Items).Map(mapProductAPIKeyToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
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
		return nil, apikey.ErrProductAPIKeyNotFound
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

// productAPIKeySearch builds the search from the request body. The product scope
// comes from the route, which the authorization middleware checked against the
// principal; the body never sets it. expires_before and expires_after are
// accepted and ignored: product API keys have no expiry column to filter on.
func productAPIKeySearch(
	productID string, body *SearchProductAPIKeysJSONRequestBody,
) repository.ProductAPIKeySearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	statuses := orEmpty(f.Status)
	q := repository.NewProductAPIKeySearch(productID)
	q = searchexec.Some(q, q.ID().In, f.Ids)
	q = searchexec.Some(q, q.Name().In, f.Names)
	q = searchexec.Some(q, q.Status().In, functional.Slice(statuses).Map(func(s ProductAPIKeyStatus) string {
		return string(s)
	}))
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.ProductAPIKeySortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
}
