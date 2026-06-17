package api

import (
	"context"

	"anchor/internal/domain/email"
	"anchor/internal/security"
)

func listLimitOffset(ctx context.Context, limit *int64, offset *int64) (string, int64, int64, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return "", 0, 0, err
	}

	var limitValue int64
	if limit != nil {
		limitValue = *limit
	}

	var offsetValue int64
	if offset != nil {
		offsetValue = *offset
	}

	return tenantID, limitValue, offsetValue, nil
}

func mapItems[T any, R any](items []T, mapper func(T) R) []R {
	out := make([]R, 0, len(items))
	for _, item := range items {
		out = append(out, mapper(item))
	}
	return out
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
	if b.Description != nil {
		in.Description = *b.Description
	}
	if b.BodyText != nil {
		in.BodyText = *b.BodyText
	}
	in.Variables = mapVariableSchemasFromAPI(b.Variables)

	tpl, err := s.EmailService.CreateTemplate(ctx, in)
	if err != nil {
		s.logger.Error().Err(err).Str("product_id", request.ProductId).Msg("failed to create email template")
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

	in := email.ListTemplatesInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Limit:     limit,
		Offset:    offset,
	}

	templates, err := s.EmailService.ListTemplates(ctx, in)
	if err != nil {
		s.logger.Error().Err(err).Str("product_id", request.ProductId).Msg("failed to list email templates")
		return nil, err
	}

	items := mapItems(templates, mapTemplateToResponse)
	return ListEmailTemplates200JSONResponse(EmailTemplateListResponse{Items: items, Count: len(items)}), nil
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
		s.logger.Error().Err(err).
			Str("product_id", request.ProductId).
			Str("email_template_id", request.EmailTemplateId).
			Msg("failed to get email template")
		return nil, err
	}
	if tpl == nil {
		return GetEmailTemplate404Response{}, nil
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
		s.logger.Error().Err(err).
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
		s.logger.Error().Err(err).
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
		s.logger.Error().Err(err).
			Str("product_id", request.ProductId).
			Str("email_template_id", request.EmailTemplateId).
			Msg("failed to get email template draft")
		return nil, err
	}
	if version == nil {
		return GetEmailTemplateDraft404Response{}, nil
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
		s.logger.Error().Err(err).
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
		s.logger.Error().Err(err).
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
	if b.UsePublished != nil {
		in.UsePublished = *b.UsePublished
	}
	if b.Variables != nil {
		in.Variables = *b.Variables
	}

	result, err := s.EmailService.Preview(ctx, in)
	if err != nil {
		s.logger.Error().Err(err).
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
		return GetEmailTemplateExamples404Response{}, nil
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
		s.logger.Error().Err(err).
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
	if b.Subject != nil {
		in.Subject = *b.Subject
	}
	if b.BodyHtml != nil {
		in.BodyHTML = *b.BodyHtml
	}
	if b.BodyText != nil {
		in.BodyText = *b.BodyText
	}
	if b.ToName != nil {
		in.ToName = *b.ToName
	}
	if b.UseDraft != nil {
		in.UseDraft = *b.UseDraft
	}
	if b.Variables != nil {
		in.Variables = *b.Variables
	}

	record, err := s.EmailService.Send(ctx, in)
	if err != nil {
		s.logger.Error().Err(err).Str("product_id", request.ProductId).Msg("failed to send email")
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

	in := email.ListSendsInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Limit:     limit,
		Offset:    offset,
	}

	records, err := s.EmailService.ListSends(ctx, in)
	if err != nil {
		s.logger.Error().Err(err).Str("product_id", request.ProductId).Msg("failed to list email sends")
		return nil, err
	}

	items := mapItems(records, mapSendRecordToResponse)
	return ListEmailSends200JSONResponse(EmailSendRecordListResponse{Items: items, Count: len(items)}), nil
}
