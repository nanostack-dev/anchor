package api

import (
	"context"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"anchor/internal/domain/license"
	"anchor/internal/security"
)

// errUsageValueMissing refuses a report that carries no value. The request
// validator runs with ExcludeRequestBody, so the contract's own required rule
// never fires, and no validate tag can stand in for it: zero is a real
// observation, so a tag that rejected the zero value would reject a true one.
// Stored, an omitted value would read as "this organization uses nothing"
// rather than as the absence of a report, which is the one reading a snapshot
// must never be given by accident.
//
// The shape matches what the validate tags produce for the other fields, so a
// consumer parses one error format for the whole body.
func errUsageValueMissing() *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:     "VALIDATION_ERROR",
		Message:  "A usage report must carry a value",
		Field:    "value",
		Metadata: map[string]any{"rule": "required"},
	}}, http.StatusBadRequest)
}

func (s *AnchorAPI) ReportOrganizationUsage(
	ctx context.Context, request ReportOrganizationUsageRequestObject,
) (ReportOrganizationUsageResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if request.Body.Value == nil {
		return nil, errUsageValueMissing()
	}

	observation, err := s.UsageService.ReportUsage(ctx, license.ReportUsageInput{
		TenantID:       tenantID,
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Key:            request.Body.Key,
		Value:          *request.Body.Value,
		From:           request.Body.From,
		To:             request.Body.To,
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
