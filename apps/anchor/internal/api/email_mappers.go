package api

import "anchor/internal/domain/email"

func mapVariableSchemasFromAPI(schemas *[]EmailVariableSchema) []email.VariableSchema {
	if schemas == nil {
		return nil
	}
	out := make([]email.VariableSchema, 0, len(*schemas))
	for _, s := range *schemas {
		vs := email.VariableSchema{
			Name: s.Name,
			Type: s.Type,
		}
		if s.Required != nil {
			vs.Required = *s.Required
		}
		if s.Help != nil {
			vs.Help = *s.Help
		}
		if s.Items != nil {
			vs.Items = mapSchemaItemsFromAPI(s.Items)
		}
		if s.Properties != nil {
			vs.Properties = mapSchemaPropsFromAPI(*s.Properties)
		}
		out = append(out, vs)
	}
	return out
}

func mapVariableSchemasToAPI(schemas []email.VariableSchema) *[]EmailVariableSchema {
	if len(schemas) == 0 {
		return nil
	}
	out := make([]EmailVariableSchema, 0, len(schemas))
	for _, s := range schemas {
		vs := EmailVariableSchema{
			Name: s.Name,
			Type: s.Type,
		}
		if s.Required {
			b := true
			vs.Required = &b
		}
		if s.Help != "" {
			h := s.Help
			vs.Help = &h
		}
		if s.Items != nil {
			vs.Items = mapSchemaItemsToAPI(s.Items)
		}
		if len(s.Properties) > 0 {
			props := mapSchemaPropsToAPI(s.Properties)
			vs.Properties = &props
		}
		out = append(out, vs)
	}
	return &out
}

func mapSchemaPropsFromAPI(props []EmailVariableSchemaProperty) []email.VariableSchemaProperty {
	out := make([]email.VariableSchemaProperty, 0, len(props))
	for _, p := range props {
		out = append(out, email.VariableSchemaProperty{Name: p.Name, Type: p.Type})
	}
	return out
}

func mapSchemaPropsToAPI(props []email.VariableSchemaProperty) []EmailVariableSchemaProperty {
	out := make([]EmailVariableSchemaProperty, 0, len(props))
	for _, p := range props {
		out = append(out, EmailVariableSchemaProperty{Name: p.Name, Type: p.Type})
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
	out := make([]email.TemplateExample, 0, len(items))
	for _, e := range items {
		out = append(out, email.TemplateExample{
			ID:        e.Id,
			Name:      e.Name,
			Variables: e.Variables,
		})
	}
	return out
}

func mapExamplesToResponse(examples []email.TemplateExample) TemplateExampleListResponse {
	items := make([]TemplateExample, 0, len(examples))
	for _, e := range examples {
		items = append(items, TemplateExample{
			Id:        e.ID,
			Name:      e.Name,
			Variables: e.Variables,
		})
	}
	return TemplateExampleListResponse{Examples: items}
}

func mapTemplateToResponse(t email.Template) EmailTemplateResponse {
	var desc *string
	if t.Description != "" {
		d := t.Description
		desc = &d
	}
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
	var bodyText *string
	if v.BodyText != "" {
		bt := v.BodyText
		bodyText = &bt
	}
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
	var toName, fromName *string
	if r.ToName != "" {
		n := r.ToName
		toName = &n
	}
	if r.FromName != "" {
		n := r.FromName
		fromName = &n
	}
	var iid *string
	if r.IntegrationInstanceID != "" {
		id := r.IntegrationInstanceID
		iid = &id
	}
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
