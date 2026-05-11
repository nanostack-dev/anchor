package api

import (
	"context"

	"github.com/nanostack-dev/shared/toolkit/search"

	"anchor/internal/domain/organization"
)

func mapOrganizationToResponse(org organization.Organization) ProductOrganizationResponse {
	return ProductOrganizationResponse{
		Id:          org.ID,
		ProductId:   org.ProductID,
		Name:        org.Name,
		Description: org.Description,
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
			ProductUserID: request.Body.FoundingMember.ProductUserId,
			RoleID:        request.Body.FoundingMember.RoleId,
		}

		res, err := s.OrganizationService.CreateWithMember(ctx, input)
		if err != nil {
			s.logger.Error().Err(err).
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
	}

	createdOrganization, err := s.OrganizationService.Create(ctx, input)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to create organization")
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
		s.logger.Error().Err(err).Msg("failed to search product organizations")
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

func (s *AnchorAPI) UpdateProductOrganization(
	ctx context.Context, request UpdateProductOrganizationRequestObject,
) (UpdateProductOrganizationResponseObject, error) {
	input := organization.UpdateOrganizationInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Name:           &request.Body.Name,
		Description:    request.Body.Description,
	}

	updatedOrganization, err := s.OrganizationService.Update(ctx, input)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to update organization")
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
