package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

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
		Items: functional.Slice(result.Items).Map(mapOrganizationLicenseSummaryToResponse),
		Total: result.Total,
		Count: result.Count,
	}, nil
}

func mapToSearchOrganizationLicensesRequest(
	body *SearchOrganizationLicensesJSONRequestBody,
) search.Request[license.SearchOrganizationLicenseFilter, license.SortFieldOrganizationLicense] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f OrganizationLicenseFilter) license.SearchOrganizationLicenseFilter {
			return license.SearchOrganizationLicenseFilter{
				OrganizationIDs:    f.OrganizationIds,
				LicenseTemplateIDs: f.LicenseTemplateIds,
				Licensed:           f.Licensed,
			}
		}).
		ToPtr()

	return search.NewRequest[license.SearchOrganizationLicenseFilter, license.SortFieldOrganizationLicense]().
		WithFilter(filter).
		WithSort(body.SortBy, body.SortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}
