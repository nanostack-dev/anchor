package api

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/workflow"
	"anchor/internal/workflow/engine"
	workflowsvc "anchor/internal/workflow/service"
)

func mapWorkflowWriteRequest(body WorkflowWriteRequest) workflow.WriteInput {
	return workflow.WriteInput{
		Name:             body.Name,
		Description:      body.Description,
		Enabled:          body.Enabled,
		TriggerEventType: body.TriggerEventType,
		Definition: workflow.Definition{
			Conditions: functional.Slice(body.Definition.Conditions).Map(mapWorkflowConditionRequest),
			Steps:      functional.Slice(body.Definition.Steps).Map(mapWorkflowStepRequest),
		},
	}
}

func mapWorkflowConditionRequest(condition WorkflowCondition) workflow.Condition {
	return workflow.Condition{
		Field:    condition.Field,
		Operator: condition.Operator,
		Value:    functional.FromPtr(condition.Value).OrElse(""),
	}
}

func mapWorkflowStepRequest(step WorkflowStep) workflow.Step {
	return workflow.Step{
		ID:              step.Id,
		Name:            functional.FromPtr(step.Name).OrElse(""),
		Action:          workflow.ActionType(step.Action),
		Params:          step.Params,
		When:            functional.Slice(functional.FromPtr(step.When).OrElse(nil)).Map(mapWorkflowConditionRequest),
		ContinueOnError: functional.FromPtr(step.ContinueOnError).OrElse(false),
	}
}

func mapWorkflowToResponse(wf workflow.Workflow) WorkflowResponse {
	return WorkflowResponse{
		Id:               wf.ID,
		Name:             wf.Name,
		Description:      wf.Description,
		Enabled:          wf.Enabled,
		TriggerEventType: wf.TriggerEventType,
		Definition: WorkflowDefinition{
			Conditions: functional.Slice(wf.Definition.Conditions).Map(mapWorkflowConditionToResponse),
			Steps:      functional.Slice(wf.Definition.Steps).Map(mapWorkflowStepToResponse),
		},
		CreatedAt: wf.CreatedAt,
		UpdatedAt: wf.UpdatedAt,
	}
}

func mapWorkflowConditionToResponse(condition workflow.Condition) WorkflowCondition {
	response := WorkflowCondition{Field: condition.Field, Operator: condition.Operator}
	if condition.Value != "" {
		response.Value = &condition.Value
	}
	return response
}

func mapWorkflowStepToResponse(step workflow.Step) WorkflowStep {
	response := WorkflowStep{
		Id:     step.ID,
		Action: string(step.Action),
		Params: step.Params,
	}
	if response.Params == nil {
		response.Params = map[string]string{}
	}
	if step.Name != "" {
		response.Name = &step.Name
	}
	if len(step.When) > 0 {
		when := []WorkflowCondition(functional.Slice(step.When).Map(mapWorkflowConditionToResponse))
		response.When = &when
	}
	if step.ContinueOnError {
		response.ContinueOnError = &step.ContinueOnError
	}
	return response
}

func mapWorkflowRuns(runs []workflow.Run) WorkflowRunListResponse {
	return WorkflowRunListResponse{
		Items: functional.Slice(runs).Map(mapWorkflowRunToResponse),
		Count: len(runs),
	}
}

func mapWorkflowRunToResponse(run workflow.Run) WorkflowRunResponse {
	response := WorkflowRunResponse{
		Id:         run.ID,
		WorkflowId: run.WorkflowID,
		EventId:    run.EventID,
		EventType:  run.EventType,
		EventData:  run.EventData,
		Trigger:    run.Trigger,
		Status:     run.Status,
		Steps:      functional.Slice(run.Steps).Map(mapWorkflowStepResultToResponse),
		Error:      run.Error,
		StartedAt:  run.StartedAt,
		FinishedAt: run.FinishedAt,
	}
	if response.EventData == nil {
		response.EventData = map[string]string{}
	}
	if run.WorkflowName != "" {
		response.WorkflowName = &run.WorkflowName
	}
	return response
}

func mapWorkflowStepResultToResponse(result workflow.StepResult) WorkflowStepResultResponse {
	response := WorkflowStepResultResponse{
		StepId: result.StepID,
		Action: string(result.Action),
		Status: result.Status,
	}
	if result.Params != nil {
		params := result.Params
		response.Params = &params
	}
	if result.Output != nil {
		output := result.Output
		response.Output = &output
	}
	if result.Error != "" {
		response.Error = &result.Error
	}
	return response
}

func mapWorkflowCatalog(catalog workflowsvc.Catalog) WorkflowCatalogResponse {
	return WorkflowCatalogResponse{
		Triggers:  functional.Slice(catalog.Triggers).Map(mapWorkflowTrigger),
		Actions:   functional.Slice(catalog.Actions).Map(mapWorkflowAction),
		Operators: catalog.Operators,
	}
}

func mapWorkflowTrigger(trigger engine.Trigger) WorkflowTriggerResponse {
	return WorkflowTriggerResponse{
		Type:        string(trigger.Type),
		Name:        trigger.Name,
		Description: trigger.Description,
		GroupType:   trigger.GroupType,
		GroupName:   trigger.GroupName,
		DataFields:  trigger.DataFields,
	}
}

func mapWorkflowAction(spec engine.ActionSpec) WorkflowActionResponse {
	return WorkflowActionResponse{
		Type:        string(spec.Type),
		Name:        spec.Name,
		Description: spec.Description,
		Group:       spec.Group,
		Writes:      spec.Writes,
		Params: functional.Slice(spec.Params).Map(func(param engine.ParamSpec) WorkflowActionParamResponse {
			response := WorkflowActionParamResponse{
				Name: param.Name, Label: param.Label, Type: param.Type, Required: param.Required,
			}
			if param.Description != "" {
				response.Description = &param.Description
			}
			return response
		}),
		Outputs: functional.Slice(spec.Outputs).Map(func(output engine.OutputSpec) WorkflowActionOutputResponse {
			return WorkflowActionOutputResponse{Name: output.Name, Description: output.Description}
		}),
	}
}
