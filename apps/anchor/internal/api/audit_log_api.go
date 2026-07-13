package api

import (
	"context"
	"encoding/json"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	"anchor/internal/domain/audit"
	"anchor/internal/security"
)

func mapAuditLogSearchRequestToInput(
	request SearchAuditLogsRequestObject,
) search.Request[audit.SearchFilter, audit.SortField] {
	var req search.Request[audit.SearchFilter, audit.SortField]

	if request.Body == nil {
		return req
	}

	var filter *audit.SearchFilter
	if request.Body.Filter != nil {
		body := request.Body.Filter
		filter = &audit.SearchFilter{
			OrganizationID: body.OrganizationId,
			Actions:        body.Actions,
			ActorTypes:     body.ActorTypes,
			ActorID:        body.ActorId,
			TargetType:     body.TargetType,
			TargetID:       body.TargetId,
			Outcome:        body.Outcome,
			CreatedAfter:   body.CreatedAfter,
			CreatedBefore:  body.CreatedBefore,
		}
	}

	// The contract defaults sort_direction to DESC (newest first) and limit
	// to 20 (max 100); the framework's WithSort/WithPagination default to
	// ASC/10, so apply the contract defaults explicitly.
	sortDirection := search.SortDescending
	if request.Body.SortDirection != nil {
		sortDirection = *request.Body.SortDirection
	}

	pagination := search.Pagination{Limit: defaultAuditLogPageSize}
	if request.Body.Pagination != nil {
		pagination = *request.Body.Pagination
	}
	if pagination.Limit <= 0 {
		pagination.Limit = defaultAuditLogPageSize
	}
	if pagination.Limit > maxAuditLogPageSize {
		pagination.Limit = maxAuditLogPageSize
	}

	return req.WithFilter(filter).
		WithSort(
			request.Body.SortBy,
			&sortDirection,
		).WithFullTextSearch(request.Body.FullTextSearch).WithPagination(
		&pagination,
	)
}

const (
	defaultAuditLogPageSize int32 = 20
	maxAuditLogPageSize     int32 = 100
)

func (s *AnchorAPI) SearchAuditLogs(
	ctx context.Context, request SearchAuditLogsRequestObject,
) (SearchAuditLogsResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	input := audit.SearchInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Request:   mapAuditLogSearchRequestToInput(request),
	}

	res, err := s.AuditLogService.Search(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Msg("failed to search audit logs")
		return nil, err
	}

	resp := AuditLogListResponse{
		Items: slicex.Map(res.Items, mapAuditLogEntryToResponse),
		Total: res.Total,
		Count: len(res.Items),
	}

	return SearchAuditLogs200JSONResponse(resp), nil
}

func mapAuditLogEntryToResponse(entry audit.Log) AuditLogResponse {
	var metadata *map[string]any
	if len(entry.MetadataJSON) > 0 {
		parsedMetadata := map[string]any{}
		if err := json.Unmarshal(entry.MetadataJSON, &parsedMetadata); err == nil {
			metadata = &parsedMetadata
		}
	}

	return AuditLogResponse{
		Id:             entry.ID,
		OrganizationId: entry.OrganizationID,
		Action:         string(entry.Action),
		Outcome:        entry.Outcome,
		ActorType:      entry.ActorType,
		ActorId:        entry.ActorID,
		ActorName:      entry.ActorName,
		TargetType:     entry.TargetType,
		TargetId:       entry.TargetID,
		TargetName:     entry.TargetName,
		RequestId:      entry.RequestID,
		Metadata:       metadata,
		CreatedAt:      entry.CreatedAt,
	}
}
