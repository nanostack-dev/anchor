package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	"anchor/internal/domain/license"
	"anchor/internal/security"
)

func (s *AnchorAPI) SearchOrganizationLicenses(
	ctx context.Context, request SearchOrganizationLicensesRequestObject,
) (SearchOrganizationLicensesResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		body = &SearchOrganizationLicensesJSONRequestBody{}
	}

	result, err := s.OrganizationLicenseService.Search(ctx, license.SearchOrganizationLicensesInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Request:   mapToSearchOrganizationLicensesRequest(body),
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Msg("failed to search organization licenses")
		return nil, err
	}

	return SearchOrganizationLicenses200JSONResponse{
		Items: slicex.Map(result.Items, mapOrganizationLicenseSummaryToResponse),
		Total: result.Total,
		Count: result.Count,
	}, nil
}

func mapToSearchOrganizationLicensesRequest(
	body *SearchOrganizationLicensesJSONRequestBody,
) search.Request[license.SearchOrganizationLicenseFilter, license.SortFieldOrganizationLicense] {
	var filter *license.SearchOrganizationLicenseFilter
	if body.Filter != nil {
		filter = &license.SearchOrganizationLicenseFilter{
			OrganizationIDs:    body.Filter.OrganizationIds,
			LicenseTemplateIDs: body.Filter.LicenseTemplateIds,
			Licensed:           body.Filter.Licensed,
		}
	}

	var request search.Request[license.SearchOrganizationLicenseFilter, license.SortFieldOrganizationLicense]
	return request.WithFilter(filter).
		WithSort(body.SortBy, body.SortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}
