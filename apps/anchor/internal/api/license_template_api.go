package api

import (
	"context"

	"anchor/internal/domain/license"
	"anchor/internal/security"
)

// ---------------------------------------------------------------------------
// License template handlers
// ---------------------------------------------------------------------------
//
// A license template is a named set of values satisfying the Product's license
// schema. Unlike the schema, a Product holds any number of them, so these
// handlers address a template by its own identifier.

func (s *AnchorAPI) CreateLicenseTemplate(
	ctx context.Context, request CreateLicenseTemplateRequestObject,
) (CreateLicenseTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	b := request.Body
	in := license.CreateTemplateInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Name:      b.Name,
		Values:    b.Values,
	}
	if b.Description != nil {
		in.Description = *b.Description
	}

	template, err := s.LicenseTemplateService.CreateTemplate(ctx, in)
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to create license template")
		return nil, err
	}
	return CreateLicenseTemplate201JSONResponse(mapLicenseTemplateToResponse(template)), nil
}

func (s *AnchorAPI) GetLicenseTemplate(
	ctx context.Context, request GetLicenseTemplateRequestObject,
) (GetLicenseTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	template, err := s.LicenseTemplateService.GetTemplate(ctx, license.GetTemplateInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		TemplateID: request.LicenseTemplateId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("license_template_id", request.LicenseTemplateId).
			Msg("failed to get license template")
		return nil, err
	}
	if template == nil {
		return GetLicenseTemplate404Response{}, nil
	}
	return GetLicenseTemplate200JSONResponse(mapLicenseTemplateToResponse(*template)), nil
}

func (s *AnchorAPI) ListLicenseTemplates(
	ctx context.Context, request ListLicenseTemplatesRequestObject,
) (ListLicenseTemplatesResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	in := license.ListTemplatesInput{TenantID: tenantID, ProductID: request.ProductId}
	if request.Params.IncludeArchived != nil {
		in.IncludeArchived = *request.Params.IncludeArchived
	}

	templates, err := s.LicenseTemplateService.ListTemplates(ctx, in)
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to list license templates")
		return nil, err
	}

	items := mapItems(templates, mapLicenseTemplateToResponse)
	return ListLicenseTemplates200JSONResponse(LicenseTemplateListResponse{
		Items: items,
		Count: len(items),
	}), nil
}

func (s *AnchorAPI) UpdateLicenseTemplate(
	ctx context.Context, request UpdateLicenseTemplateRequestObject,
) (UpdateLicenseTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	template, err := s.LicenseTemplateService.UpdateTemplate(ctx, license.UpdateTemplateInput{
		TenantID:    tenantID,
		ProductID:   request.ProductId,
		TemplateID:  request.LicenseTemplateId,
		Name:        request.Body.Name,
		Description: request.Body.Description,
		// A nil Values leaves the set alone; a supplied one replaces it, so an
		// omitted license field is an unset rather than a no-op.
		Values: request.Body.Values,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("license_template_id", request.LicenseTemplateId).
			Msg("failed to update license template")
		return nil, err
	}
	return UpdateLicenseTemplate200JSONResponse(mapLicenseTemplateToResponse(template)), nil
}

func (s *AnchorAPI) ArchiveLicenseTemplate(
	ctx context.Context, request ArchiveLicenseTemplateRequestObject,
) (ArchiveLicenseTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if err = s.LicenseTemplateService.ArchiveTemplate(ctx, license.ArchiveTemplateInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		TemplateID: request.LicenseTemplateId,
	}); err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("license_template_id", request.LicenseTemplateId).
			Msg("failed to archive license template")
		return nil, err
	}
	return ArchiveLicenseTemplate204Response{}, nil
}
