package api

import (
	"context"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
)

// defaultLicenseHistoryLimit mirrors the openapi.yaml `limit` default for
// getOrganizationLicenseHistory. oapi-codegen leaves an optional query
// parameter's default undeserialized — a nil pointer, not the schema
// default — so it is applied here, the same way usage_api.go applies its own.
const defaultLicenseHistoryLimit int32 = 50

// ---------------------------------------------------------------------------
// Organization license handlers
// ---------------------------------------------------------------------------
//
// A license is a singleton on its Organization, so it has no identifier in any
// path — the Organization is the only address it has.

func (s *AnchorAPI) InstantiateOrganizationLicense(
	ctx context.Context, request InstantiateOrganizationLicenseRequestObject,
) (InstantiateOrganizationLicenseResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	organizationLicense, err := s.OrganizationLicenseService.Instantiate(
		ctx, license.InstantiateLicenseInput{
			TenantID:       tenantID,
			ProductID:      request.ProductId,
			OrganizationID: request.OrganizationId,
			TemplateID:     request.Body.TemplateId,
		},
	)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to instantiate organization license",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}
	return InstantiateOrganizationLicense201JSONResponse(
		mapOrganizationLicenseToResponse(organizationLicense),
	), nil
}

func (s *AnchorAPI) GetOrganizationLicense(
	ctx context.Context, request GetOrganizationLicenseRequestObject,
) (GetOrganizationLicenseResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	organizationLicense, err := s.OrganizationLicenseService.GetLicense(ctx, license.GetLicenseInput{
		TenantID:       tenantID,
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to get organization license",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}
	if organizationLicense == nil {
		return GetOrganizationLicense404JSONResponse{NotFoundJSONResponse(
			notFoundBody("ORGANIZATION_LICENSE_NOT_FOUND", "Organization License does not exist."),
		)}, nil
	}
	return GetOrganizationLicense200JSONResponse(
		mapOrganizationLicenseReadToResponse(*organizationLicense),
	), nil
}

func (s *AnchorAPI) AdjustOrganizationLicense(
	ctx context.Context, request AdjustOrganizationLicenseRequestObject,
) (AdjustOrganizationLicenseResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	organizationLicense, err := s.OrganizationLicenseService.AdjustValues(
		ctx, license.AdjustLicenseInput{
			TenantID:       tenantID,
			ProductID:      request.ProductId,
			OrganizationID: request.OrganizationId,
			Values:         request.Body.Values,
		},
	)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to adjust organization license",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}
	return AdjustOrganizationLicense200JSONResponse(
		mapOrganizationLicenseToResponse(organizationLicense),
	), nil
}

func (s *AnchorAPI) GetOrganizationLicenseDiff(
	ctx context.Context, request GetOrganizationLicenseDiffRequestObject,
) (GetOrganizationLicenseDiffResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	diff, err := s.OrganizationLicenseService.DiffAgainstTemplate(ctx, license.GetLicenseInput{
		TenantID:       tenantID,
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to diff organization license against its template",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}
	return GetOrganizationLicenseDiff200JSONResponse(mapLicenseDiffToResponse(diff)), nil
}

func (s *AnchorAPI) GetOrganizationLicenseHistory(
	ctx context.Context, request GetOrganizationLicenseHistoryRequestObject,
) (GetOrganizationLicenseHistoryResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	q := licenserepo.NewOrganizationLicenseChangeSearch(licenserepo.OrganizationLicenseChangeScope{
		PlatformTenantID: tenantID, ProductID: request.ProductId, OrganizationID: request.OrganizationId,
	}).
		SortBy(licenserepo.OrganizationLicenseChangeSortChangedAt, search.SortDescending).
		Limit(ptr.DerefOr(request.Params.Limit, defaultLicenseHistoryLimit)).
		Offset(ptr.DerefOr(request.Params.Offset, int32(0)))
	result, err := s.LicenseHistoryService.SearchChanges(ctx, q)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to get organization license history",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}

	return GetOrganizationLicenseHistory200JSONResponse(OrganizationLicenseHistoryResponse{
		Items:      functional.Slice(result.Items).Map(mapOrganizationLicenseChangeToResponse),
		Total:      result.Total,
		Count:      result.Count,
		NextCursor: nextCursor(result.NextCursor),
	}), nil
}

func (s *AnchorAPI) SearchOrganizationLicenseHistory(
	ctx context.Context, request SearchOrganizationLicenseHistoryRequestObject,
) (SearchOrganizationLicenseHistoryResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		body = &SearchOrganizationLicenseHistoryJSONRequestBody{}
	}

	if err = validateLicenseChangesSearch(tenantID, request.ProductId, request.OrganizationId, body); err != nil {
		return nil, err
	}
	q := licenseChangeSearch(licenserepo.OrganizationLicenseChangeScope{
		PlatformTenantID: tenantID, ProductID: request.ProductId, OrganizationID: request.OrganizationId,
	}, body)
	result, err := s.LicenseHistoryService.SearchChanges(ctx, q)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search organization license history",
			slog.String("product_id", request.ProductId),
			slog.String("organization_id", request.OrganizationId),
		)
		return nil, err
	}

	return SearchOrganizationLicenseHistory200JSONResponse(OrganizationLicenseHistoryResponse{
		Items:      functional.Slice(result.Items).Map(mapOrganizationLicenseChangeToResponse),
		Total:      result.Total,
		Count:      result.Count,
		NextCursor: nextCursor(result.NextCursor),
	}), nil
}

// licenseChangeSearch builds the search from the request body. The scope comes
// from the authenticated principal and the route; the body never sets it.
// changed_after is inclusive and changed_before exclusive. full_text_search is
// accepted and ignored: a history entry has no text column that the contract names.
func licenseChangeSearch(
	scope licenserepo.OrganizationLicenseChangeScope, body *SearchOrganizationLicenseHistoryJSONRequestBody,
) licenserepo.OrganizationLicenseChangeSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	q := licenserepo.NewOrganizationLicenseChangeSearch(scope)
	q = searchexec.Some(q, q.ChangeType().In, f.Types)
	q = searchexec.Opt(q, q.ChangedAt().Since, f.ChangedAfter)
	q = searchexec.Opt(q, q.ChangedAt().Before, f.ChangedBefore)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		licenserepo.OrganizationLicenseChangeSortChangedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
}
