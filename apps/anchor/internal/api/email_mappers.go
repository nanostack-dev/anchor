package api

import (
	"anchor/internal/domain/email"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

func mapVariableSchemasFromAPI(schemas *[]EmailVariableSchema) []email.VariableSchema {
	if schemas == nil {
		return nil
	}
	out := functional.Slice(*schemas).Map(func(s EmailVariableSchema) email.VariableSchema {
		vs := email.VariableSchema{
			Name:     s.Name,
			Type:     s.Type,
			Required: functional.FromPtr(s.Required).OrElse(false),
			Help:     functional.FromPtr(s.Help).OrElse(""),
		}
		if s.Items != nil {
			vs.Items = mapSchemaItemsFromAPI(s.Items)
		}
		if s.Properties != nil {
			vs.Properties = mapSchemaPropsFromAPI(*s.Properties)
		}
		return vs
	}).ToSlice()
	if out == nil {
		out = []email.VariableSchema{}
	}
	return out
}

func mapVariableSchemasToAPI(schemas []email.VariableSchema) *[]EmailVariableSchema {
	if len(schemas) == 0 {
		return nil
	}
	out := functional.Slice(schemas).Map(func(s email.VariableSchema) EmailVariableSchema {
		vs := EmailVariableSchema{
			Name:     s.Name,
			Type:     s.Type,
			Required: functional.OptionOf(s.Required, s.Required).ToPtr(),
			Help:     functional.OptionOf(s.Help, s.Help != "").ToPtr(),
		}
		if s.Items != nil {
			vs.Items = mapSchemaItemsToAPI(s.Items)
		}
		if len(s.Properties) > 0 {
			props := mapSchemaPropsToAPI(s.Properties)
			vs.Properties = &props
		}
		return vs
	}).ToSlice()
	return &out
}

func mapSchemaPropsFromAPI(props []EmailVariableSchemaProperty) []email.VariableSchemaProperty {
	out := functional.Slice(props).Map(func(p EmailVariableSchemaProperty) email.VariableSchemaProperty {
		return email.VariableSchemaProperty{Name: p.Name, Type: p.Type}
	}).ToSlice()
	if out == nil {
		out = []email.VariableSchemaProperty{}
	}
	return out
}

func mapSchemaPropsToAPI(props []email.VariableSchemaProperty) []EmailVariableSchemaProperty {
	out := functional.Slice(props).Map(func(p email.VariableSchemaProperty) EmailVariableSchemaProperty {
		return EmailVariableSchemaProperty{Name: p.Name, Type: p.Type}
	}).ToSlice()
	if out == nil {
		out = []EmailVariableSchemaProperty{}
	}
	return out
}

func mapSchemaItemsFromAPI(items *EmailVariableSchemaItems) *email.VariableSchemaItems {
	if items == nil {
		return nil
	}
	result := &email.VariableSchemaItems{Type: items.Type}
	if items.Properties != nil {
		result.Properties = mapSchemaPropsFromAPI(*items.Properties)
	}
	return result
}

func mapSchemaItemsToAPI(items *email.VariableSchemaItems) *EmailVariableSchemaItems {
	if items == nil {
		return nil
	}
	result := &EmailVariableSchemaItems{Type: items.Type}
	if len(items.Properties) > 0 {
		props := mapSchemaPropsToAPI(items.Properties)
		result.Properties = &props
	}
	return result
}

func mapExamplesFromAPI(items []TemplateExample) []email.TemplateExample {
	out := functional.Slice(items).Map(func(e TemplateExample) email.TemplateExample {
		return email.TemplateExample{
			ID:        e.Id,
			Name:      e.Name,
			Variables: e.Variables,
		}
	}).ToSlice()
	if out == nil {
		out = []email.TemplateExample{}
	}
	return out
}

func mapExamplesToResponse(examples []email.TemplateExample) TemplateExampleListResponse {
	items := functional.Slice(examples).Map(func(e email.TemplateExample) TemplateExample {
		return TemplateExample{
			Id:        e.ID,
			Name:      e.Name,
			Variables: e.Variables,
		}
	}).ToSlice()
	if items == nil {
		items = []TemplateExample{}
	}
	return TemplateExampleListResponse{Examples: items}
}

func mapTemplateToResponse(t email.Template) EmailTemplateResponse {
	desc := functional.OptionOf(t.Description, t.Description != "").ToPtr()
	return EmailTemplateResponse{
		Id:                 t.ID,
		ProductId:          t.ProductID,
		Slug:               t.Slug,
		Name:               t.Name,
		Description:        desc,
		DraftVersionId:     t.DraftVersionID,
		PublishedVersionId: t.PublishedVersionID,
		IsActive:           t.IsActive,
		CreatedAt:          t.CreatedAt,
		UpdatedAt:          t.UpdatedAt,
	}
}

func mapTemplateVersionToResponse(v email.TemplateVersion) EmailTemplateVersionResponse {
	bodyText := functional.OptionOf(v.BodyText, v.BodyText != "").ToPtr()
	return EmailTemplateVersionResponse{
		Id:            v.ID,
		TemplateId:    v.TemplateID,
		VersionNumber: int(v.VersionNumber),
		Subject:       v.Subject,
		BodyHtml:      v.BodyHTML,
		BodyText:      bodyText,
		Variables:     mapVariableSchemasToAPI(v.Variables),
		Status:        v.Status,
		PublishedAt:   v.PublishedAt,
		CreatedAt:     v.CreatedAt,
		UpdatedAt:     v.UpdatedAt,
	}
}

func mapSendRecordToResponse(r email.SendRecord) EmailSendRecordResponse {
	toName := functional.OptionOf(r.ToName, r.ToName != "").ToPtr()
	fromName := functional.OptionOf(r.FromName, r.FromName != "").ToPtr()
	iid := functional.OptionOf(r.IntegrationInstanceID, r.IntegrationInstanceID != "").ToPtr()
	return EmailSendRecordResponse{
		Id:                    r.ID,
		ProductId:             r.ProductID,
		IntegrationInstanceId: iid,
		TemplateId:            r.TemplateID,
		TemplateVersionId:     r.TemplateVersionID,
		DedupeKey:             r.DedupeKey,
		ToAddress:             r.ToAddress,
		ToName:                toName,
		FromAddress:           r.FromAddress,
		FromName:              fromName,
		Subject:               r.Subject,
		Status:                r.Status,
		LastError:             r.LastError,
		SentAt:                r.SentAt,
		CreatedAt:             r.CreatedAt,
		UpdatedAt:             r.UpdatedAt,
	}
}
