package api

import (
	"context"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

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

// defaultUsageSeriesLimit mirrors the openapi.yaml `limit` default for
// getOrganizationUsageSeries. oapi-codegen leaves an optional query
// parameter's default undeserialized — a nil pointer, not the schema
// default — so it is applied here.
const defaultUsageSeriesLimit int32 = 50

func usageSeriesPagination(limit *int32, offset *int32) search.Pagination {
	return search.Pagination{
		Limit:  ptr.DerefOr(limit, defaultUsageSeriesLimit),
		Offset: ptr.DerefOr(offset, int32(0)),
	}
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

func (s *AnchorAPI) GetOrganizationUsageSeries(
	ctx context.Context, request GetOrganizationUsageSeriesRequestObject,
) (GetOrganizationUsageSeriesResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	from := request.Params.From
	result, err := s.UsageSeriesService.GetSeries(ctx, license.GetUsageSeriesInput{
		TenantID:       tenantID,
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Key:            request.Params.Key,
		Granularity:    request.Params.Granularity,
		From:           &from,
		To:             request.Params.To,
		Pagination:     usageSeriesPagination(request.Params.Limit, request.Params.Offset),
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Str("key", request.Params.Key).
			Msg("failed to get organization usage series")
		return nil, err
	}

	items := slicex.Map(result.Items, mapUsageSeriesPointToResponse)
	return GetOrganizationUsageSeries200JSONResponse(UsageSeriesResponse{
		Items: items,
		Total: result.Total,
		Count: result.Count,
	}), nil
}
