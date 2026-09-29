package api

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"anchor/internal/domain/organizationinvitation"
)

func (s *AnchorAPI) GetInvitationSettings(
	ctx context.Context, request GetInvitationSettingsRequestObject,
) (GetInvitationSettingsResponseObject, error) {
	settings, err := s.InvitationSettingsService.Get(ctx, organizationinvitation.GetSettingsInput{
		ProductID: request.ProductId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Msg("failed to get invitation settings")
		return nil, err
	}

	return GetInvitationSettings200JSONResponse(mapInvitationSettingsToResponse(settings)), nil
}

func (s *AnchorAPI) UpdateInvitationSettings(
	ctx context.Context, request UpdateInvitationSettingsRequestObject,
) (UpdateInvitationSettingsResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	settings, err := s.InvitationSettingsService.Update(ctx, organizationinvitation.UpdateSettingsInput{
		ProductID:            request.ProductId,
		InvitationDelivery:   request.Body.InvitationDelivery,
		EmailTemplateID:      request.Body.EmailTemplateId,
		AcceptURLTemplate:    request.Body.AcceptUrlTemplate,
		DefaultExpirySeconds: request.Body.DefaultExpirySeconds,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Msg("failed to update invitation settings")
		return nil, err
	}

	return UpdateInvitationSettings200JSONResponse(mapInvitationSettingsToResponse(settings)), nil
}

func mapInvitationSettingsToResponse(settings organizationinvitation.Settings) InvitationSettingsResponse {
	return InvitationSettingsResponse{
		InvitationDelivery:   settings.InvitationDelivery,
		EmailTemplateId:      settings.EmailTemplateID,
		AcceptUrlTemplate:    settings.AcceptURLTemplate,
		DefaultExpirySeconds: int64(settings.DefaultExpiry / time.Second),
	}
}
