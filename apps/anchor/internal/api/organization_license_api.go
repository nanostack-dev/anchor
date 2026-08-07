package api

import (
	"context"

	"anchor/internal/domain/license"
	"anchor/internal/security"
)

// ---------------------------------------------------------------------------
// Organization license handlers
// ---------------------------------------------------------------------------
//
// An Organization's license is its own copy of a template's values, and it is a
// singleton on that Organization. It therefore has no identifier in any path —
// the Organization is the only address it has.

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
		return GetOrganizationLicense404Response{}, nil
	}
	return GetOrganizationLicense200JSONResponse(
		mapOrganizationLicenseToResponse(*organizationLicense),
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
			// Merged into what the license holds, so a license field absent from
			// the request keeps its value rather than being unset.
			Values: request.Body.Values,
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
