package api

import (
	"context"

	"anchor/internal/domain/license"
	"anchor/internal/security"
)

// ---------------------------------------------------------------------------
// Usage handlers
// ---------------------------------------------------------------------------
//
// Usage hangs off the Organization's license path because that is what it is
// measured against, but it needs no license to exist: the key is resolved
// against the Product's license schema.

func (s *AnchorAPI) ReportOrganizationUsage(
	ctx context.Context, request ReportOrganizationUsageRequestObject,
) (ReportOrganizationUsageResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	observation, err := s.UsageService.ReportUsage(ctx, license.ReportUsageInput{
		TenantID:       tenantID,
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Key:            request.Body.Key,
		Value:          request.Body.Value,
		WindowStart:    request.Body.WindowStart,
		WindowEnd:      request.Body.WindowEnd,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Str("key", request.Body.Key).
			Msg("failed to report organization usage")
		return nil, err
	}
	return ReportOrganizationUsage201JSONResponse(
		mapUsageObservationToResponse(observation),
	), nil
}
