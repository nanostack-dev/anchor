package mapper

import (
	"encoding/json"
	"time"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/email"
)

type EmailTemplateMapper struct{}

func NewEmailTemplateMapper() *EmailTemplateMapper {
	return &EmailTemplateMapper{}
}

func (m *EmailTemplateMapper) ToDomain(entity model.EmailTemplates) email.Template {
	examples := []email.TemplateExample{}
	if entity.ExampleData != "" && entity.ExampleData != "[]" {
		_ = json.Unmarshal([]byte(entity.ExampleData), &examples)
	}
	return email.Template{
		ID:                 entity.ID,
		PlatformTenantID:   entity.PlatformTenantID,
		ProductID:          entity.ProductID,
		Slug:               entity.Slug,
		Name:               entity.Name,
		Description:        entity.Description,
		DraftVersionID:     entity.DraftVersionID,
		PublishedVersionID: entity.PublishedVersionID,
		IsActive:           entity.IsActive,
		Examples:           examples,
		CreatedAt:          entity.CreatedAt,
		UpdatedAt:          entity.UpdatedAt,
	}
}

func (m *EmailTemplateMapper) ToEntity(domain email.Template) model.EmailTemplates {
	examplesJSON := "[]"
	if len(domain.Examples) > 0 {
		if b, err := json.Marshal(domain.Examples); err == nil {
			examplesJSON = string(b)
		}
	}
	return model.EmailTemplates{
		ID:                 domain.ID,
		PlatformTenantID:   domain.PlatformTenantID,
		ProductID:          domain.ProductID,
		Slug:               domain.Slug,
		Name:               domain.Name,
		Description:        domain.Description,
		DraftVersionID:     domain.DraftVersionID,
		PublishedVersionID: domain.PublishedVersionID,
		IsActive:           domain.IsActive,
		ExampleData:        examplesJSON,
		CreatedAt:          domain.CreatedAt,
		UpdatedAt:          domain.UpdatedAt,
	}
}

type EmailTemplateVersionMapper struct{}

func NewEmailTemplateVersionMapper() *EmailTemplateVersionMapper {
	return &EmailTemplateVersionMapper{}
}

func (m *EmailTemplateVersionMapper) ToDomain(entity model.EmailTemplateVersions) email.TemplateVersion {
	vars := []email.VariableSchema{}
	if entity.VariablesJSON != "" {
		_ = json.Unmarshal([]byte(entity.VariablesJSON), &vars)
	}
	return email.TemplateVersion{
		ID:            entity.ID,
		TemplateID:    entity.TemplateID,
		VersionNumber: entity.VersionNumber,
		Subject:       entity.Subject,
		BodyHTML:      entity.BodyHTML,
		BodyText:      entity.BodyText,
		Variables:     vars,
		Status:        email.TemplateVersionStatus(entity.Status),
		CreatedAt:     entity.CreatedAt,
		UpdatedAt:     entity.UpdatedAt,
		PublishedAt:   entity.PublishedAt,
	}
}

func (m *EmailTemplateVersionMapper) ToEntity(domain email.TemplateVersion) model.EmailTemplateVersions {
	varsJSON := "[]"
	if len(domain.Variables) > 0 {
		if b, err := json.Marshal(domain.Variables); err == nil {
			varsJSON = string(b)
		}
	}
	if domain.CreatedAt.IsZero() {
		domain.CreatedAt = time.Now()
	}
	if domain.UpdatedAt.IsZero() {
		domain.UpdatedAt = domain.CreatedAt
	}
	return model.EmailTemplateVersions{
		ID:            domain.ID,
		TemplateID:    domain.TemplateID,
		VersionNumber: domain.VersionNumber,
		Subject:       domain.Subject,
		BodyHTML:      domain.BodyHTML,
		BodyText:      domain.BodyText,
		VariablesJSON: varsJSON,
		Status:        string(domain.Status),
		CreatedAt:     domain.CreatedAt,
		UpdatedAt:     domain.UpdatedAt,
		PublishedAt:   domain.PublishedAt,
	}
}
