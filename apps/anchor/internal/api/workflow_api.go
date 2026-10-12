package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/workflow"
	"anchor/internal/security"
)

func (s *AnchorAPI) GetWorkflowCatalog(
	ctx context.Context, request GetWorkflowCatalogRequestObject,
) (GetWorkflowCatalogResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	catalog, err := s.WorkflowService.Catalog(ctx, workflow.ListInput{TenantID: tenantID, ProductID: request.ProductId})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to read workflow catalog")
		return nil, err
	}
	return GetWorkflowCatalog200JSONResponse(mapWorkflowCatalog(catalog)), nil
}

func (s *AnchorAPI) ListWorkflows(
	ctx context.Context, request ListWorkflowsRequestObject,
) (ListWorkflowsResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	workflows, err := s.WorkflowService.List(ctx, workflow.ListInput{
		TenantID:  tenantID,
		ProductID: request.ProductId,
		Include:   functional.FromPtr(request.Params.Include).OrElse(nil),
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to list workflows")
		return nil, err
	}
	return ListWorkflows200JSONResponse(WorkflowListResponse{
		Items: functional.Slice(workflows).Map(s.mapWorkflowToResponse),
		Count: len(workflows),
	}), nil
}

func (s *AnchorAPI) CreateWorkflow(
	ctx context.Context, request CreateWorkflowRequestObject,
) (CreateWorkflowResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	created, err := s.WorkflowService.Create(ctx, workflow.CreateInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		WriteInput: mapWorkflowWriteRequest(*request.Body),
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to create workflow")
		return nil, err
	}
	return CreateWorkflow201JSONResponse(s.mapWorkflowToResponse(created)), nil
}

func (s *AnchorAPI) GetWorkflow(
	ctx context.Context, request GetWorkflowRequestObject,
) (GetWorkflowResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	found, err := s.WorkflowService.Get(ctx, workflow.GetInput{
		TenantID: tenantID, ProductID: request.ProductId, WorkflowID: request.WorkflowId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("workflow_id", request.WorkflowId).
			Msg("failed to get workflow")
		return nil, err
	}
	return GetWorkflow200JSONResponse(s.mapWorkflowToResponse(found)), nil
}

func (s *AnchorAPI) UpdateWorkflow(
	ctx context.Context, request UpdateWorkflowRequestObject,
) (UpdateWorkflowResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := s.WorkflowService.Update(ctx, workflow.UpdateInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		WorkflowID: request.WorkflowId,
		WriteInput: mapWorkflowWriteRequest(*request.Body),
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("workflow_id", request.WorkflowId).
			Msg("failed to update workflow")
		return nil, err
	}
	return UpdateWorkflow200JSONResponse(s.mapWorkflowToResponse(updated)), nil
}

func (s *AnchorAPI) DeleteWorkflow(
	ctx context.Context, request DeleteWorkflowRequestObject,
) (DeleteWorkflowResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.WorkflowService.Delete(ctx, workflow.DeleteInput{
		TenantID: tenantID, ProductID: request.ProductId, WorkflowID: request.WorkflowId,
	}); err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("workflow_id", request.WorkflowId).
			Msg("failed to delete workflow")
		return nil, err
	}
	return DeleteWorkflow204Response{}, nil
}

func (s *AnchorAPI) ListWorkflowRuns(
	ctx context.Context, request ListWorkflowRunsRequestObject,
) (ListWorkflowRunsResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	runs, err := s.WorkflowService.ListRuns(ctx, workflow.ListRunsInput{
		TenantID: tenantID, ProductID: request.ProductId, WorkflowID: request.WorkflowId,
		Limit: limitParam(request.Params.Limit),
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("workflow_id", request.WorkflowId).
			Msg("failed to list workflow runs")
		return nil, err
	}
	return ListWorkflowRuns200JSONResponse(mapWorkflowRuns(runs)), nil
}

func (s *AnchorAPI) ListProductWorkflowRuns(
	ctx context.Context, request ListProductWorkflowRunsRequestObject,
) (ListProductWorkflowRunsResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	runs, err := s.WorkflowService.ListProductRuns(ctx, workflow.ListProductRunsInput{
		TenantID: tenantID, ProductID: request.ProductId, Limit: limitParam(request.Params.Limit),
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to list product workflow runs")
		return nil, err
	}
	return ListProductWorkflowRuns200JSONResponse(mapWorkflowRuns(runs)), nil
}

func (s *AnchorAPI) RunWorkflow(
	ctx context.Context, request RunWorkflowRequestObject,
) (RunWorkflowResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.WorkflowService.Run(ctx, workflow.RunInput{
		TenantID: tenantID, ProductID: request.ProductId, WorkflowID: request.WorkflowId,
		EventData: request.Body.EventData,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("workflow_id", request.WorkflowId).
			Msg("failed to run workflow")
		return nil, err
	}
	return RunWorkflow200JSONResponse(mapWorkflowRunToResponse(run)), nil
}

func (s *AnchorAPI) DryRunWorkflow(
	ctx context.Context, request DryRunWorkflowRequestObject,
) (DryRunWorkflowResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	run, err := s.WorkflowService.DryRun(ctx, workflow.DryRunInput{
		TenantID:   tenantID,
		ProductID:  request.ProductId,
		WriteInput: mapWorkflowWriteRequest(request.Body.Workflow),
		EventData:  request.Body.EventData,
	})
	if err != nil {
		logAPIError(s.logger, err).Str("product_id", request.ProductId).Msg("failed to dry run workflow")
		return nil, err
	}
	return DryRunWorkflow200JSONResponse(mapWorkflowRunToResponse(run)), nil
}

func limitParam(limit *int32) int {
	if limit == nil {
		return 0
	}
	return int(*limit)
}
