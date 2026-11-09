package api

import (
	"context"
	"log/slog"

	"anchor/internal/domain/email"
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	emailrepo "anchor/internal/email/repository"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
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
		log.ErrorTo(ctx, s.logger, err, "failed to create email template",
			slog.String("product_id", request.ProductId),
		)
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

	scope := emailrepo.EmailTemplateScope{PlatformTenantID: tenantID, ProductID: request.ProductId}
	templates, err := legacyListPage(limit, offset,
		func(pagination search.Pagination) (search.Result[email.Template], error) {
			q := emailrepo.NewEmailTemplateSearch(scope).SortBy(emailrepo.EmailTemplateSortCreatedAt, search.SortDescending)
			return s.EmailService.SearchTemplates(ctx, searchexec.APIPage(q.Query, &pagination))
		})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to list email templates",
			slog.String("product_id", request.ProductId),
		)
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

	q, err := emailTemplateSearch(
		emailrepo.EmailTemplateScope{PlatformTenantID: tenantID, ProductID: request.ProductId}, body,
	)
	if err != nil {
		return nil, err
	}
	result, err := s.EmailService.SearchTemplates(ctx, q)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search email templates",
			slog.String("product_id", request.ProductId),
		)
		return nil, err
	}

	return SearchEmailTemplates200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapTemplateToResponse),
		Total:      result.Total,
		Count:      result.Count,
		NextCursor: nextCursor(result.NextCursor),
	}, nil
}

// emailTemplateSearch builds the search from the request body. The scope comes
// from the authenticated principal and the route; the body never sets it.
func emailTemplateSearch(
	scope emailrepo.EmailTemplateScope, body *SearchEmailTemplatesJSONRequestBody,
) (emailrepo.EmailTemplateSearch, error) {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	q := emailrepo.NewEmailTemplateSearch(scope)
	q = searchexec.Some(q, q.ID().In, f.Ids)
	q = searchexec.Some(q, q.Slug().In, f.Slugs)
	q = searchexec.Opt(q, q.IsActive().Eq, f.IsActive)
	// full_text_search matches the name and the slug.
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		emailrepo.EmailTemplateSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination), nil
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
		log.ErrorTo(ctx, s.logger, err, "failed to get email template",
			slog.String("product_id", request.ProductId),
			slog.String("email_template_id", request.EmailTemplateId),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to update email template",
			slog.String("product_id", request.ProductId),
			slog.String("email_template_id", request.EmailTemplateId),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to delete email template",
			slog.String("product_id", request.ProductId),
			slog.String("email_template_id", request.EmailTemplateId),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to get email template draft",
			slog.String("product_id", request.ProductId),
			slog.String("email_template_id", request.EmailTemplateId),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to update email template draft",
			slog.String("product_id", request.ProductId),
			slog.String("email_template_id", request.EmailTemplateId),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to publish email template",
			slog.String("product_id", request.ProductId),
			slog.String("email_template_id", request.EmailTemplateId),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to preview email template",
			slog.String("product_id", request.ProductId),
			slog.String("email_template_id", request.EmailTemplateId),
		)
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
		log.ErrorTo(ctx, s.logger, err, "failed to save template examples",
			slog.String("product_id", request.ProductId),
			slog.String("email_template_id", request.EmailTemplateId),
		)
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
	if _, isProductKey := security.GetProductAPIKeyCaller(ctx); in.UseDraft && isProductKey {
		return nil, fault.Forbidden("EMAIL_DRAFT_SEND_FORBIDDEN",
			"A product API key sends the published version; use_draft needs a platform user.")
	}

	record, err := s.EmailService.Send(ctx, in)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to send email", slog.String("product_id", request.ProductId))
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

	scope := emailrepo.EmailSendScope{PlatformTenantID: tenantID, ProductID: request.ProductId}
	records, err := legacyListPage(limit, offset,
		func(pagination search.Pagination) (search.Result[email.SendRecord], error) {
			q := emailrepo.NewEmailSendSearch(scope).SortBy(emailrepo.EmailSendSortCreatedAt, search.SortDescending)
			if request.Params.TemplateId != nil {
				q = q.TemplateID().In(*request.Params.TemplateId)
			}
			if request.Params.Status != nil {
				q = q.Status().In(*request.Params.Status)
			}
			return s.EmailService.SearchSends(ctx, searchexec.APIPage(q.Query, &pagination))
		})
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to list email sends", slog.String("product_id", request.ProductId))
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

	q, err := emailSendSearch(
		emailrepo.EmailSendScope{PlatformTenantID: tenantID, ProductID: request.ProductId}, body,
	)
	if err != nil {
		return nil, err
	}
	result, err := s.EmailService.SearchSends(ctx, q)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search email sends",
			slog.String("product_id", request.ProductId),
		)
		return nil, err
	}

	return SearchEmailSends200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapSendRecordToResponse),
		Total:      result.Total,
		Count:      result.Count,
		NextCursor: nextCursor(result.NextCursor),
	}, nil
}

// emailSendSearch builds the search from the request body. The scope comes
// from the authenticated principal and the route; the body never sets it.
// full_text_search is accepted and ignored: the send history has no text
// column that the contract names.
func emailSendSearch(
	scope emailrepo.EmailSendScope, body *SearchEmailSendsJSONRequestBody,
) (emailrepo.EmailSendSearch, error) {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	if err := requireOneOf("statuses", f.Statuses,
		email.SendStatusQueued, email.SendStatusSending, email.SendStatusSent, email.SendStatusFailed); err != nil {
		return emailrepo.EmailSendSearch{}, err
	}
	q := emailrepo.NewEmailSendSearch(scope)
	q = searchexec.Some(q, q.TemplateID().In, f.TemplateIds)
	q = searchexec.Some(q, q.Status().In, f.Statuses)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		emailrepo.EmailSendSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination), nil
}
