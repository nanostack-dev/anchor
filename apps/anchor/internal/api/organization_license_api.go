package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/license"
	"anchor/internal/security"
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
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to instantiate organization license")
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
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to get organization license")
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
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to adjust organization license")
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
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to diff organization license against its template")
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

	result, err := s.LicenseHistoryService.ListChanges(ctx, license.ListLicenseChangesInput{
		TenantID:       tenantID,
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Pagination: search.Pagination{
			Limit:  ptr.DerefOr(request.Params.Limit, defaultLicenseHistoryLimit),
			Offset: ptr.DerefOr(request.Params.Offset, int32(0)),
		},
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to get organization license history")
		return nil, err
	}

	return GetOrganizationLicenseHistory200JSONResponse(OrganizationLicenseHistoryResponse{
		Items: functional.Slice(result.Items).Map(mapOrganizationLicenseChangeToResponse),
		Total: result.Total,
		Count: result.Count,
	}), nil
}
