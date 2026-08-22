package mapper

import (
	"encoding/json"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/email"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

type EmailSendRecordMapper struct{}

func NewEmailSendRecordMapper() *EmailSendRecordMapper { return &EmailSendRecordMapper{} }

func (m *EmailSendRecordMapper) ToDomain(entity model.EmailSendRecords) email.SendRecord {
	integrationID := functional.FromPtr(entity.IntegrationInstanceID).OrElse("")
	return email.SendRecord{
		ID:                    entity.ID,
		PlatformTenantID:      entity.PlatformTenantID,
		ProductID:             entity.ProductID,
		IntegrationInstanceID: integrationID,
		TemplateID:            entity.TemplateID,
		TemplateVersionID:     entity.TemplateVersionID,
		DedupeKey:             entity.DedupeKey,
		ToAddress:             entity.ToAddress,
		ToName:                entity.ToName,
		FromAddress:           entity.FromAddress,
		FromName:              entity.FromName,
		Subject:               entity.Subject,
		BodyHTML:              entity.BodyHTML,
		BodyText:              entity.BodyText,
		VariablesJSON:         json.RawMessage(entity.VariablesJSON),
		MessageID:             entity.MessageID,
		Status:                email.SendStatus(entity.Status),
		LastError:             entity.LastError,
		SentAt:                entity.SentAt,
		CreatedAt:             entity.CreatedAt,
		UpdatedAt:             entity.UpdatedAt,
	}
}

func (m *EmailSendRecordMapper) ToEntity(domain email.SendRecord) model.EmailSendRecords {
	integrationID := functional.OptionOf(domain.IntegrationInstanceID, domain.IntegrationInstanceID != "").ToPtr()
	varsJSON := "{}"
	if len(domain.VariablesJSON) > 0 {
		varsJSON = string(domain.VariablesJSON)
	}
	return model.EmailSendRecords{
		ID:                    domain.ID,
		PlatformTenantID:      domain.PlatformTenantID,
		ProductID:             domain.ProductID,
		IntegrationInstanceID: integrationID,
		TemplateID:            domain.TemplateID,
		TemplateVersionID:     domain.TemplateVersionID,
		DedupeKey:             domain.DedupeKey,
		ToAddress:             domain.ToAddress,
		ToName:                domain.ToName,
		FromAddress:           domain.FromAddress,
		FromName:              domain.FromName,
		Subject:               domain.Subject,
		BodyHTML:              domain.BodyHTML,
		BodyText:              domain.BodyText,
		VariablesJSON:         varsJSON,
		MessageID:             domain.MessageID,
		Status:                string(domain.Status),
		LastError:             domain.LastError,
		SentAt:                domain.SentAt,
		CreatedAt:             domain.CreatedAt,
		UpdatedAt:             domain.UpdatedAt,
	}
}
