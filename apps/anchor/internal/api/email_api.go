package api

import (
	"context"

	"anchor/internal/domain/email"
	"anchor/internal/searchx"
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
)

func listLimitOffset(ctx context.Context, limit *int64, offset *int64) (string, int64, int64, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return "", 0, 0, err
	}

	limitValue := ptr.DerefOr(limit, int64(0))
	offsetValue := ptr.DerefOr(offset, int64(0))

	return tenantID, limitValue, offsetValue, nil
}

// ---------------------------------------------------------------------------
// Template handlers
// ---------------------------------------------------------------------------

func (s *AnchorAPI) CreateEmailTemplate(
	ctx context.Context, request CreateEmailTemplateRequestObject,
) (CreateEmailTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	b := request.Body
	in := email.CreateTemplateInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Slug:      b.Slug,
		Name:      b.Name,
		Subject:   b.Subject,
		BodyHTML:  b.BodyHtml,
	}
	in.Description = functional.FromPtr(b.Description).OrElse("")
	in.BodyText = functional.FromPtr(b.BodyText).OrElse("")
	in.Variables = mapVariableSchemasFromAPI(b.Variables)

	tpl, err := s.EmailService.CreateTemplate(ctx, in)
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to create email template")
		return nil, err
	}
	return CreateEmailTemplate201JSONResponse(mapTemplateToResponse(tpl)), nil
}

func (s *AnchorAPI) ListEmailTemplates(
	ctx context.Context, request ListEmailTemplatesRequestObject,
) (ListEmailTemplatesResponseObject, error) {
	tenantID, limit, offset, err := listLimitOffset(ctx, request.Params.Limit, request.Params.Offset)
	if err != nil {
		return nil, err
	}

	templates, err := legacyListPage(limit, offset,
		func(pagination search.Pagination) (search.Result[email.Template], error) {
			return s.EmailService.SearchTemplates(ctx, email.SearchTemplatesInput{
				TenantID:  tenantID,
				ProductID: request.ProductId,
				Request: search.NewRequest[email.SearchTemplatesFilter, email.SortFieldTemplate]().
					WithSort(new(email.SortFieldTemplateCreatedAt), new(search.SortDescending)).
					WithPagination(&pagination),
			})
		})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to list email templates")
		return nil, err
	}

	items := functional.Slice(templates.Items).Map(mapTemplateToResponse)
	return ListEmailTemplates200JSONResponse(EmailTemplateListResponse{Items: items, Count: int(templates.Total)}), nil
}

func (s *AnchorAPI) SearchEmailTemplates(
	ctx context.Context, request SearchEmailTemplatesRequestObject,
) (SearchEmailTemplatesResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		body = &SearchEmailTemplatesJSONRequestBody{}
	}

	result, err := s.EmailService.SearchTemplates(ctx, email.SearchTemplatesInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Request:   mapToSearchEmailTemplatesRequest(body),
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to search email templates")
		return nil, err
	}

	return SearchEmailTemplates200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapTemplateToResponse),
		Total:      result.Total,
		Count:      result.Count,
		NextCursor: nextCursor(result.NextCursor),
	}, nil
}

func mapToSearchEmailTemplatesRequest(
	body *SearchEmailTemplatesJSONRequestBody,
) search.Request[email.SearchTemplatesFilter, email.SortFieldTemplate] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f EmailTemplateFilter) email.SearchTemplatesFilter {
			return email.SearchTemplatesFilter{IDs: f.Ids, Slugs: f.Slugs, IsActive: f.IsActive}
		}).
		ToPtr()
	sortBy, sortDirection := searchx.Sort(
		body.SortBy, body.SortDirection, email.SortFieldTemplateCreatedAt, search.SortDescending,
	)

	return search.NewRequest[email.SearchTemplatesFilter, email.SortFieldTemplate]().
		WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}

func (s *AnchorAPI) GetEmailTemplate(
	ctx context.Context, request GetEmailTemplateRequestObject,
) (GetEmailTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tpl, err := s.EmailService.GetTemplate(ctx, email.GetTemplateInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		ID:        request.EmailTemplateId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("email_template_id", request.EmailTemplateId).
			Msg("failed to get email template")
		return nil, err
	}
	if tpl == nil {
		return GetEmailTemplate404JSONResponse{NotFoundJSONResponse(
			notFoundBody("EMAIL_TEMPLATE_NOT_FOUND", "Email Template does not exist."),
		)}, nil
	}
	return GetEmailTemplate200JSONResponse(mapTemplateToResponse(*tpl)), nil
}

func (s *AnchorAPI) UpdateEmailTemplate(
	ctx context.Context, request UpdateEmailTemplateRequestObject,
) (UpdateEmailTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tpl, err := s.EmailService.UpdateTemplate(ctx, email.UpdateTemplateInput{
		TenantID:    tenantID,
		ProductID:   request.ProductId,
		ID:          request.EmailTemplateId,
		Name:        request.Body.Name,
		Description: request.Body.Description,
		IsActive:    request.Body.IsActive,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("email_template_id", request.EmailTemplateId).
			Msg("failed to update email template")
		return nil, err
	}
	return UpdateEmailTemplate200JSONResponse(mapTemplateToResponse(tpl)), nil
}

func (s *AnchorAPI) DeleteEmailTemplate(
	ctx context.Context, request DeleteEmailTemplateRequestObject,
) (DeleteEmailTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	err = s.EmailService.DeleteTemplate(ctx, email.DeleteTemplateInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		ID:        request.EmailTemplateId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("email_template_id", request.EmailTemplateId).
			Msg("failed to delete email template")
		return nil, err
	}
	return DeleteEmailTemplate204Response{}, nil
}

func (s *AnchorAPI) GetEmailTemplateDraft(
	ctx context.Context, request GetEmailTemplateDraftRequestObject,
) (GetEmailTemplateDraftResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	version, err := s.EmailService.GetTemplateDraft(ctx, email.GetTemplateDraftInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		TemplateID: request.EmailTemplateId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("email_template_id", request.EmailTemplateId).
			Msg("failed to get email template draft")
		return nil, err
	}
	if version == nil {
		return GetEmailTemplateDraft404JSONResponse{NotFoundJSONResponse(
			notFoundBody("EMAIL_TEMPLATE_DRAFT_NOT_FOUND", "Email Template Draft does not exist."),
		)}, nil
	}
	return GetEmailTemplateDraft200JSONResponse(mapTemplateVersionToResponse(*version)), nil
}

func (s *AnchorAPI) UpdateEmailTemplateDraft(
	ctx context.Context, request UpdateEmailTemplateDraftRequestObject,
) (UpdateEmailTemplateDraftResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	b := request.Body
	in := email.UpdateTemplateDraftInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		TemplateID: request.EmailTemplateId,
		Subject:    b.Subject,
		BodyHTML:   b.BodyHtml,
		BodyText:   b.BodyText,
	}
	if b.Variables != nil {
		vars := mapVariableSchemasFromAPI(b.Variables)
		in.Variables = &vars
	}

	version, err := s.EmailService.UpdateTemplateDraft(ctx, in)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("email_template_id", request.EmailTemplateId).
			Msg("failed to update email template draft")
		return nil, err
	}
	return UpdateEmailTemplateDraft200JSONResponse(mapTemplateVersionToResponse(version)), nil
}

func (s *AnchorAPI) PublishEmailTemplate(
	ctx context.Context, request PublishEmailTemplateRequestObject,
) (PublishEmailTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	version, err := s.EmailService.PublishTemplate(ctx, email.PublishTemplateInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		TemplateID: request.EmailTemplateId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("email_template_id", request.EmailTemplateId).
			Msg("failed to publish email template")
		return nil, err
	}
	return PublishEmailTemplate200JSONResponse(mapTemplateVersionToResponse(version)), nil
}

func (s *AnchorAPI) PreviewEmailTemplate(
	ctx context.Context, request PreviewEmailTemplateRequestObject,
) (PreviewEmailTemplateResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	b := request.Body
	in := email.PreviewInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		TemplateID: request.EmailTemplateId,
	}
	in.UsePublished = functional.FromPtr(b.UsePublished).OrElse(false)
	in.Variables = functional.FromPtr(b.Variables).OrElse(nil)

	result, err := s.EmailService.Preview(ctx, in)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("email_template_id", request.EmailTemplateId).
			Msg("failed to preview email template")
		return nil, err
	}
	warnings := result.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return PreviewEmailTemplate200JSONResponse(EmailTemplatePreviewResponse{
		Subject:  result.Subject,
		BodyHtml: result.BodyHTML,
		BodyText: result.BodyText,
		Warnings: warnings,
	}), nil
}

func (s *AnchorAPI) GetEmailTemplateExamples(
	ctx context.Context, request GetEmailTemplateExamplesRequestObject,
) (GetEmailTemplateExamplesResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tpl, err := s.EmailService.GetTemplate(ctx, email.GetTemplateInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		ID:        request.EmailTemplateId,
	})
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return GetEmailTemplateExamples404JSONResponse{NotFoundJSONResponse(
			notFoundBody("EMAIL_TEMPLATE_EXAMPLES_NOT_FOUND", "Email Template Examples does not exist."),
		)}, nil
	}
	return GetEmailTemplateExamples200JSONResponse(mapExamplesToResponse(tpl.Examples)), nil
}

func (s *AnchorAPI) SaveEmailTemplateExamples(
	ctx context.Context, request SaveEmailTemplateExamplesRequestObject,
) (SaveEmailTemplateExamplesResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	examples := mapExamplesFromAPI(request.Body.Examples)
	saved, err := s.EmailService.SaveTemplateExamples(ctx, email.SaveTemplateExamplesInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		TemplateID: request.EmailTemplateId,
		Examples:   examples,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("email_template_id", request.EmailTemplateId).
			Msg("failed to save template examples")
		return nil, err
	}
	return SaveEmailTemplateExamples200JSONResponse(mapExamplesToResponse(saved)), nil
}

// ---------------------------------------------------------------------------
// Send handlers
// ---------------------------------------------------------------------------

func (s *AnchorAPI) SendEmail(
	ctx context.Context, request SendEmailRequestObject,
) (SendEmailResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	b := request.Body
	in := email.SendInput{
		TenantID:     tenantID,
		ProductID:    request.ProductId,
		TemplateID:   b.TemplateId,
		TemplateSlug: b.TemplateSlug,
		ToAddress:    string(b.ToAddress),
		DedupeKey:    b.DedupeKey,
	}
	in.ToName = functional.FromPtr(b.ToName).OrElse("")
	in.UseDraft = functional.FromPtr(b.UseDraft).OrElse(false)
	in.Variables = functional.FromPtr(b.Variables).OrElse(nil)

	record, err := s.EmailService.Send(ctx, in)
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to send email")
		return nil, err
	}
	return SendEmail201JSONResponse(mapSendRecordToResponse(record)), nil
}

func (s *AnchorAPI) ListEmailSends(
	ctx context.Context, request ListEmailSendsRequestObject,
) (ListEmailSendsResponseObject, error) {
	tenantID, limit, offset, err := listLimitOffset(ctx, request.Params.Limit, request.Params.Offset)
	if err != nil {
		return nil, err
	}

	var filter email.SearchSendsFilter
	if request.Params.TemplateId != nil {
		filter.TemplateIDs = []string{*request.Params.TemplateId}
	}
	if request.Params.Status != nil {
		filter.Statuses = []email.SendStatus{*request.Params.Status}
	}

	records, err := legacyListPage(limit, offset,
		func(pagination search.Pagination) (search.Result[email.SendRecord], error) {
			return s.EmailService.SearchSends(ctx, email.SearchSendsInput{
				TenantID:  tenantID,
				ProductID: request.ProductId,
				Request: search.NewRequest[email.SearchSendsFilter, email.SortFieldSend]().
					WithFilter(&filter).
					WithSort(new(email.SortFieldSendCreatedAt), new(search.SortDescending)).
					WithPagination(&pagination),
			})
		})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to list email sends")
		return nil, err
	}

	items := functional.Slice(records.Items).Map(mapSendRecordToResponse)
	return ListEmailSends200JSONResponse(EmailSendRecordListResponse{Items: items, Count: int(records.Total)}), nil
}

func (s *AnchorAPI) SearchEmailSends(
	ctx context.Context, request SearchEmailSendsRequestObject,
) (SearchEmailSendsResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		body = &SearchEmailSendsJSONRequestBody{}
	}

	result, err := s.EmailService.SearchSends(ctx, email.SearchSendsInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Request:   mapToSearchEmailSendsRequest(body),
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to search email sends")
		return nil, err
	}

	return SearchEmailSends200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapSendRecordToResponse),
		Total:      result.Total,
		Count:      result.Count,
		NextCursor: nextCursor(result.NextCursor),
	}, nil
}

func mapToSearchEmailSendsRequest(
	body *SearchEmailSendsJSONRequestBody,
) search.Request[email.SearchSendsFilter, email.SortFieldSend] {
	filter := functional.FromPtr(body.Filter).
		Map(func(f EmailSendFilter) email.SearchSendsFilter {
			return email.SearchSendsFilter{TemplateIDs: f.TemplateIds, Statuses: f.Statuses}
		}).
		ToPtr()
	sortBy, sortDirection := searchx.Sort(
		body.SortBy, body.SortDirection, email.SortFieldSendCreatedAt, search.SortDescending,
	)

	return search.NewRequest[email.SearchSendsFilter, email.SortFieldSend]().
		WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}
