package api

import (
	"context"
	"encoding/json"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/organization"
)

// mapMetadataToResponse decodes stored organization metadata for the API
// response. Metadata is written through a validating service path, so a value
// that no longer decodes is treated as absent rather than failing the read.
func mapMetadataToResponse(metadataJSON json.RawMessage) *Metadata {
	if len(metadataJSON) == 0 {
		return nil
	}

	var metadata Metadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil
	}

	return &metadata
}

// mapMetadataToInput converts a request-body metadata object into the plain map
// the service layer validates.
func mapMetadataToInput(metadata *Metadata) map[string]any {
	if metadata == nil {
		return nil
	}
	return *metadata
}

func mapOrganizationToResponse(org organization.Organization) ProductOrganizationResponse {
	return ProductOrganizationResponse{
		Id:          org.ID,
		ProductId:   org.ProductID,
		Name:        org.Name,
		Description: org.Description,
		Metadata:    mapMetadataToResponse(org.MetadataJSON),
		CreatedAt:   org.CreatedAt,
		UpdatedAt:   org.UpdatedAt,
	}
}

func (s *AnchorAPI) CreateProductOrganization(
	ctx context.Context, request CreateProductOrganizationRequestObject,
) (CreateProductOrganizationResponseObject, error) {
	if request.Body.FoundingMember != nil {
		input := organization.CreateOrganizationWithMemberInput{
			ProductID:     request.ProductId,
			Name:          request.Body.Name,
			Description:   request.Body.Description,
			Metadata:      mapMetadataToInput(request.Body.Metadata),
			ProductUserID: request.Body.FoundingMember.ProductUserId,
			RoleID:        request.Body.FoundingMember.RoleId,
		}

		res, err := s.OrganizationService.CreateWithMember(ctx, input)
		if err != nil {
			logAPIError(s.logger, err).
				Str("product_id", request.ProductId).
				Str("product_user_id", request.Body.FoundingMember.ProductUserId).
				Str("role_id", request.Body.FoundingMember.RoleId).
				Msg("failed to create organization with founding member")
			return nil, err
		}

		return CreateProductOrganization201JSONResponse(mapOrganizationToResponse(res.Organization)), nil
	}

	input := organization.CreateOrganizationInput{
		ProductID:   request.ProductId,
		Name:        request.Body.Name,
		Description: request.Body.Description,
		Metadata:    mapMetadataToInput(request.Body.Metadata),
	}

	createdOrganization, err := s.OrganizationService.Create(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to create organization")
		return nil, err
	}

	return CreateProductOrganization201JSONResponse(mapOrganizationToResponse(createdOrganization)), nil
}

func (s *AnchorAPI) SearchProductOrganizations(
	ctx context.Context, request SearchProductOrganizationsRequestObject,
) (SearchProductOrganizationsResponseObject, error) {
	searchReqBody := request.Body
	if searchReqBody == nil {
		searchReqBody = &SearchProductOrganizationsJSONRequestBody{}
	}

	searchRequest := mapToSearchProductOrganizationInput(searchReqBody)

	input := organization.SearchProductOrganizationsInput{
		ProductID: request.ProductId,
		Request:   searchRequest,
	}

	result, err := s.OrganizationService.Search(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to search product organizations")
		return nil, err
	}

	organizations := make([]ProductOrganizationResponse, len(result.Items))
	for i, item := range result.Items {
		organizations[i] = mapOrganizationToResponse(item)
	}

	return SearchProductOrganizations200JSONResponse{
		Items: organizations,
		Total: result.Total,
		Count: result.Count,
	}, nil
}

func (s *AnchorAPI) GetProductOrganization(
	ctx context.Context, request GetProductOrganizationRequestObject,
) (GetProductOrganizationResponseObject, error) {
	org, err := s.OrganizationService.Find(ctx, organization.FindOrganizationInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to get organization")
		return nil, err
	}
	if org == nil {
		return GetProductOrganization404Response{}, nil
	}

	return GetProductOrganization200JSONResponse(mapOrganizationToResponse(*org)), nil
}

func (s *AnchorAPI) DeleteProductOrganization(
	ctx context.Context, request DeleteProductOrganizationRequestObject,
) (DeleteProductOrganizationResponseObject, error) {
	err := s.OrganizationService.Delete(ctx, organization.DeleteOrganizationInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to delete organization")
		return nil, err
	}

	return DeleteProductOrganization204Response{}, nil
}

func (s *AnchorAPI) UpdateProductOrganization(
	ctx context.Context, request UpdateProductOrganizationRequestObject,
) (UpdateProductOrganizationResponseObject, error) {
	input := organization.UpdateOrganizationInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Name:           &request.Body.Name,
		Description:    request.Body.Description,
		Metadata:       mapMetadataToInput(request.Body.Metadata),
	}

	updatedOrganization, err := s.OrganizationService.Update(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to update organization")
		return nil, err
	}

	return UpdateProductOrganization200JSONResponse(mapOrganizationToResponse(updatedOrganization)), nil
}

func mapToSearchProductOrganizationInput(
	searchReqBody *SearchProductOrganizationsJSONRequestBody,
) search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization] {
	var filter *organization.SearchProductOrganizationFilter
	if searchReqBody.Filter != nil {
		filter = &organization.SearchProductOrganizationFilter{
			IDs:   searchReqBody.Filter.Ids,
			Names: searchReqBody.Filter.Names,
		}
	}

	var req search.Request[organization.SearchProductOrganizationFilter, organization.SortFieldProductOrganization]
	return req.WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(
		searchReqBody.Pagination,
	)
}
