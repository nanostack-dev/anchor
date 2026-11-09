package api

import (
	"context"
	"log/slog"

	"anchor/internal/domain/license"
	"anchor/internal/security"

	licenserepo "anchor/internal/license/repository"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
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
	in.Description = functional.FromPtr(b.Description).OrElse("")

	template, err := s.LicenseTemplateService.CreateTemplate(ctx, in)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to create license template",
			slog.String("product_id", request.ProductId),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to get license template",
			slog.String("product_id", request.ProductId),
			slog.String("license_template_id", request.LicenseTemplateId),
		)
		return nil, err
	}
	if template == nil {
		return GetLicenseTemplate404JSONResponse{NotFoundJSONResponse(
			notFoundBody("LICENSE_TEMPLATE_NOT_FOUND", "License Template does not exist."),
		)}, nil
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

	templates, err := s.LicenseTemplateService.ListTemplates(ctx, license.ListTemplatesInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Status:    request.Params.Status,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to list license templates",
			slog.String("product_id", request.ProductId),
		)
		return nil, err
	}

	items := functional.Slice(templates).Map(mapLicenseTemplateToResponse)
	return ListLicenseTemplates200JSONResponse(LicenseTemplateListResponse{
		Items: items,
		Count: len(items),
	}), nil
}

func (s *AnchorAPI) SearchLicenseTemplates(
	ctx context.Context, request SearchLicenseTemplatesRequestObject,
) (SearchLicenseTemplatesResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		body = &SearchLicenseTemplatesJSONRequestBody{}
	}

	q, err := licenseTemplateSearch(
		licenserepo.LicenseTemplateScope{PlatformTenantID: tenantID, ProductID: request.ProductId}, body,
	)
	if err != nil {
		return nil, err
	}
	result, err := s.LicenseTemplateService.SearchTemplates(ctx, q)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search license templates",
			slog.String("product_id", request.ProductId),
		)
		return nil, err
	}

	return SearchLicenseTemplates200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapLicenseTemplateToResponse),
		Total:      result.Total,
		Count:      result.Count,
		NextCursor: nextCursor(result.NextCursor),
	}, nil
}

// licenseTemplateSearch builds the search from the request body. The scope
// comes from the authenticated principal and the route; the body never sets it.
func licenseTemplateSearch(
	scope licenserepo.LicenseTemplateScope, body *SearchLicenseTemplatesJSONRequestBody,
) (licenserepo.LicenseTemplateSearch, error) {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	if err := requireOneOf("statuses", f.Statuses, license.TemplateActive, license.TemplateArchived); err != nil {
		return licenserepo.LicenseTemplateSearch{}, err
	}
	q := licenserepo.NewLicenseTemplateSearch(scope)
	q = searchexec.Some(q, q.Status().In, f.Statuses)
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		licenserepo.LicenseTemplateSortName, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination), nil
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
		log.ErrorTo(ctx, s.logger, err, "failed to update license template",
			slog.String("product_id", request.ProductId),
			slog.String("license_template_id", request.LicenseTemplateId),
		)
		return nil, err
	}
	return UpdateLicenseTemplate200JSONResponse(mapLicenseTemplateToResponse(template)), nil
}

func (s *AnchorAPI) DeleteLicenseTemplate(
	ctx context.Context, request DeleteLicenseTemplateRequestObject,
) (DeleteLicenseTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if err = s.LicenseTemplateService.DeleteTemplate(ctx, license.DeleteTemplateInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		TemplateID: request.LicenseTemplateId,
	}); err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to delete license template",
			slog.String("product_id", request.ProductId),
			slog.String("license_template_id", request.LicenseTemplateId),
		)
		return nil, err
	}
	return DeleteLicenseTemplate204Response{}, nil
}

func (s *AnchorAPI) ArchiveLicenseTemplate(
	ctx context.Context, request ArchiveLicenseTemplateRequestObject,
) (ArchiveLicenseTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	template, err := s.LicenseTemplateService.ArchiveTemplate(ctx, license.ArchiveTemplateInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		TemplateID: request.LicenseTemplateId,
	})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to archive license template",
			slog.String("product_id", request.ProductId),
			slog.String("license_template_id", request.LicenseTemplateId),
		)
		return nil, err
	}
	return ArchiveLicenseTemplate200JSONResponse(mapLicenseTemplateToResponse(template)), nil
}
