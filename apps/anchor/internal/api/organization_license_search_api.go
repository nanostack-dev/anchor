package api

import (
	"context"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	licenserepo "anchor/internal/license/repository"
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
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

	q := organizationLicenseSearch(
		licenserepo.OrganizationLicenseScope{PlatformTenantID: tenantID, ProductID: request.ProductId}, body,
	)
	result, err := s.OrganizationLicenseService.Search(ctx, q)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search organization licenses",
			slog.String("product_id", request.ProductId),
		)
		return nil, err
	}

	return SearchOrganizationLicenses200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapOrganizationLicenseSummaryToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
		Count:      result.Count,
	}, nil
}

// organizationLicenseSearch builds the search from the request body. The scope
// comes from the authenticated principal and the route; the body never sets it.
func organizationLicenseSearch(
	scope licenserepo.OrganizationLicenseScope, body *SearchOrganizationLicensesJSONRequestBody,
) licenserepo.OrganizationLicenseSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	q := licenserepo.NewOrganizationLicenseSearch(scope)
	q = searchexec.Some(q, q.ID().In, f.OrganizationIds)
	q = searchexec.Some(q, q.TemplateID().In, f.LicenseTemplateIds)
	q = searchexec.Opt(q, q.Licensed().Is, f.Licensed)
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		licenserepo.OrganizationLicenseSortOrganizationName, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
}
