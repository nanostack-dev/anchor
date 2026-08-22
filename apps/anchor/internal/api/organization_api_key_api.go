package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	orgapikey "anchor/internal/domain/organization/apikey"
)

func mapOrganizationAPIKeyToResponse(
	organizationAPIKey orgapikey.OrganizationAPIKey,
) OrganizationAPIKeyResponse {
	return OrganizationAPIKeyResponse{
		Id:              organizationAPIKey.ID,
		OrganizationId:  organizationAPIKey.OrganizationID,
		Name:            organizationAPIKey.Name,
		Description:     organizationAPIKey.Description,
		ObfuscatedValue: organizationAPIKey.ObfuscatedValue,
		ExpiresAt:       organizationAPIKey.ExpiresAt,
		LastUsedAt:      organizationAPIKey.LastUsedAt,
		Status:          organizationAPIKey.Status,
		CreatedAt:       organizationAPIKey.CreatedAt,
		UpdatedAt:       organizationAPIKey.UpdatedAt,
		Permissions: functional.Slice(
			organizationAPIKey.Permissions).Map(

			func(perm orgapikey.OrganizationAPIKeyPermission) OrganizationAPIKeyPermissionResponse {
				return OrganizationAPIKeyPermissionResponse{
					OrganizationApiKeyId: perm.APIKeyID,
					OrganizationId:       perm.OrganizationID,
					ProductId:            perm.ProductID,
					PermissionName:       perm.PermissionName,
				}
			}),
	}
}

func mapCreatedOrganizationAPIKeyToResponse(
	organizationAPIKey orgapikey.OrganizationAPIKey,
	clearValue string,
) CreatedOrganizationAPIKeyResponse {
	baseResponse := mapOrganizationAPIKeyToResponse(organizationAPIKey)
	return CreatedOrganizationAPIKeyResponse{
		Id:              baseResponse.Id,
		OrganizationId:  baseResponse.OrganizationId,
		Name:            baseResponse.Name,
		Description:     baseResponse.Description,
		ObfuscatedValue: baseResponse.ObfuscatedValue,
		ExpiresAt:       baseResponse.ExpiresAt,
		LastUsedAt:      baseResponse.LastUsedAt,
		Status:          baseResponse.Status,
		CreatedAt:       baseResponse.CreatedAt,
		UpdatedAt:       baseResponse.UpdatedAt,
		Permissions:     baseResponse.Permissions,
		Value:           clearValue,
	}
}

func (s *AnchorAPI) CreateOrganizationAPIKey(
	ctx context.Context,
	request CreateOrganizationAPIKeyRequestObject,
) (CreateOrganizationAPIKeyResponseObject, error) {
	input := orgapikey.CreateOrganizationAPIKeyInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Name:           request.Body.Name,
		Description:    request.Body.Description,
		ExpiresAt:      request.Body.ExpiresAt,
		Permissions:    request.Body.Permissions,
	}

	organizationAPIKey, clearValue, err := s.OrganizationAPIKeyService.Create(ctx, input)
	if err != nil {
		return nil, err
	}

	response := mapCreatedOrganizationAPIKeyToResponse(organizationAPIKey, clearValue)
	return CreateOrganizationAPIKey201JSONResponse(response), nil
}

func (s *AnchorAPI) SearchOrganizationAPIKeys(
	ctx context.Context,
	request SearchOrganizationAPIKeysRequestObject,
) (SearchOrganizationAPIKeysResponseObject, error) {
	searchRequest := mapToSearchOrganizationAPIKeyInput(request.Body)

	input := orgapikey.SearchOrganizationAPIKeysInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Request:        searchRequest,
	}

	result, err := s.OrganizationAPIKeyService.Search(ctx, input)
	if err != nil {
		return nil, err
	}

	response := OrganizationAPIKeyListResponse{
		Count: result.Count,
		Items: functional.Slice(result.Items).Map(mapOrganizationAPIKeyToResponse),
		Total: result.Total,
	}

	return SearchOrganizationAPIKeys200JSONResponse(response), nil
}

func (s *AnchorAPI) GetOrganizationAPIKey(
	ctx context.Context,
	request GetOrganizationAPIKeyRequestObject,
) (GetOrganizationAPIKeyResponseObject, error) {
	input := orgapikey.GetOrganizationAPIKeyInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		ID:             request.ApiKeyId,
	}

	organizationAPIKey, err := s.OrganizationAPIKeyService.GetByID(ctx, input)
	if err != nil {
		return nil, err
	}

	response := mapOrganizationAPIKeyToResponse(*organizationAPIKey)
	return GetOrganizationAPIKey200JSONResponse(response), nil
}

func (s *AnchorAPI) UpdateOrganizationAPIKey(
	ctx context.Context,
	request UpdateOrganizationAPIKeyRequestObject,
) (UpdateOrganizationAPIKeyResponseObject, error) {
	input := orgapikey.UpdateOrganizationAPIKeyInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		ID:             request.ApiKeyId,
		Name:           &request.Body.Name,
		Description:    request.Body.Description,
		Status:         request.Body.Status,
	}

	updated, err := s.OrganizationAPIKeyService.Update(ctx, input)
	if err != nil {
		return nil, err
	}

	response := mapOrganizationAPIKeyToResponse(updated)
	return UpdateOrganizationAPIKey200JSONResponse(response), nil
}

func (s *AnchorAPI) DeleteOrganizationAPIKey(
	ctx context.Context,
	request DeleteOrganizationAPIKeyRequestObject,
) (DeleteOrganizationAPIKeyResponseObject, error) {
	input := orgapikey.DeleteOrganizationAPIKeyInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		ID:             request.ApiKeyId,
	}

	err := s.OrganizationAPIKeyService.Delete(ctx, input)
	if err != nil {
		return nil, err
	}

	return DeleteOrganizationAPIKey204Response{}, nil
}

func (s *AnchorAPI) ValidateOrganizationAPIKey(
	ctx context.Context,
	request ValidateOrganizationAPIKeyRequestObject,
) (ValidateOrganizationAPIKeyResponseObject, error) {
	input := orgapikey.ValidateOrganizationAPIKeyScopesInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Scopes:         request.Body.RequiredScopes,
		APIKeyValue:    request.Body.ApiKey,
	}

	result, err := s.OrganizationAPIKeyService.ValidateAPIKeyAndScopes(ctx, input)
	if err != nil {
		return nil, err
	}
	permissions := functional.Slice(
		result.APIKey.Permissions).Map(

		func(permission orgapikey.OrganizationAPIKeyPermission) string {
			return permission.PermissionName
		})

	if result.Inactive || len(result.MissingPrivileges) > 0 {
		response := OrganizationAPIKeyValidateResponse{
			ApiKey:            mapOrganizationAPIKeyToResponse(result.APIKey),
			Permissions:       permissions,
			MissingPrivileges: result.MissingPrivileges,
		}
		return validateOrganizationAPIKeyForbiddenResponse{response}, nil
	}
	response := OrganizationAPIKeyValidateResponse{
		ApiKey:            mapOrganizationAPIKeyToResponse(result.APIKey),
		Permissions:       permissions,
		MissingPrivileges: result.MissingPrivileges,
	}

	return ValidateOrganizationAPIKey200JSONResponse(response), nil
}

type validateOrganizationAPIKeyForbiddenResponse struct {
	OrganizationAPIKeyValidateResponse
}

func (response validateOrganizationAPIKeyForbiddenResponse) VisitValidateOrganizationAPIKeyResponse(
	w http.ResponseWriter,
) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)

	return json.NewEncoder(w).Encode(response.OrganizationAPIKeyValidateResponse)
}

// IntrospectOrganizationAPIKey resolves a raw organization API key within the
// product without an organization id, returning the key's organization,
// identity, and permissions. When required scopes are supplied they are checked
// and any missing scopes yield a 403.
func (s *AnchorAPI) IntrospectOrganizationAPIKey(
	ctx context.Context,
	request IntrospectOrganizationAPIKeyRequestObject,
) (IntrospectOrganizationAPIKeyResponseObject, error) {
	var scopes []string
	if request.Body.RequiredScopes != nil {
		scopes = *request.Body.RequiredScopes
	}

	input := orgapikey.IntrospectOrganizationAPIKeyInput{
		ProductID:   request.ProductId,
		Scopes:      scopes,
		APIKeyValue: request.Body.ApiKey,
	}

	result, err := s.OrganizationAPIKeyService.IntrospectAPIKey(ctx, input)
	if err != nil {
		return nil, err
	}

	permissions := functional.Slice(
		result.APIKey.Permissions).Map(

		func(permission orgapikey.OrganizationAPIKeyPermission) string {
			return permission.PermissionName
		})

	if result.Inactive || len(result.MissingPrivileges) > 0 {
		return IntrospectOrganizationAPIKey403JSONResponse{
			ApiKey:            mapOrganizationAPIKeyToResponse(result.APIKey),
			Permissions:       permissions,
			MissingPrivileges: result.MissingPrivileges,
		}, nil
	}

	return IntrospectOrganizationAPIKey200JSONResponse{
		ApiKey:            mapOrganizationAPIKeyToResponse(result.APIKey),
		Permissions:       permissions,
		MissingPrivileges: result.MissingPrivileges,
	}, nil
}

func mapToSearchOrganizationAPIKeyInput(
	searchReqBody *SearchOrganizationAPIKeysJSONRequestBody,
) search.Request[
	orgapikey.SearchOrganizationAPIKeyFilter,
	orgapikey.SortFieldOrganizationAPIKey,
] {
	var filter *orgapikey.SearchOrganizationAPIKeyFilter
	if searchReqBody.Filter != nil {
		filter = &orgapikey.SearchOrganizationAPIKeyFilter{
			OrganizationAPIKeyIDs: searchReqBody.Filter.Ids,
			Names:                 searchReqBody.Filter.Names,
			LastUsedBefore:        searchReqBody.Filter.LastUsedBefore,
			LastUsedAfter:         searchReqBody.Filter.LastUsedAfter,
		}
		if searchReqBody.Filter.Status != nil {
			filter.Status = functional.Slice(
				*searchReqBody.Filter.Status).Map(

				func(s OrganizationAPIKeyStatus) string {
					return string(s)
				})
		}
	}

	var req search.Request[
		orgapikey.SearchOrganizationAPIKeyFilter,
		orgapikey.SortFieldOrganizationAPIKey,
	]
	return req.WithFilter(filter).
		WithSort(searchReqBody.SortBy, searchReqBody.SortDirection).
		WithFullTextSearch(searchReqBody.FullTextSearch).
		WithPagination(searchReqBody.Pagination)
}
