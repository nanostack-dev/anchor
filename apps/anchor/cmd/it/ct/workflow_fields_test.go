package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/workflow"
)

func typedEmitStep(id, name, data, types string) ct.WorkflowStep {
	return step(id, "workflow.emit", map[string]string{"event": name, "data": data, "data_types": types})
}

func (w workflowWorld) catalog() ct.WorkflowCatalogResponse {
	w.t.Helper()
	resp, err := w.client.GetWorkflowCatalogWithResponse(context.Background(), w.product.ProductID)
	require.NoError(w.t, err)
	require.Equal(w.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	return *resp.JSON200
}

func TestWorkflowCatalog_TypesEveryEventFieldAndStepOutput(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)

	catalog := w.catalog()

	for _, trigger := range catalog.Triggers {
		assert.Len(t, trigger.Fields, len(trigger.DataFields), "trigger %s", trigger.Type)
		for _, field := range trigger.Fields {
			assert.NotEmpty(t, field.Type, "trigger %s field %s", trigger.Type, field.Name)
		}
		if trigger.Type == "organization.created" {
			assert.Equal(t, []ct.WorkflowEventFieldResponse{{
				Name: "organization_id", Type: ct.WorkflowFieldTypeOrganization,
				Description: "Identifier of the organization.",
			}}, trigger.Fields)
		}
	}
	for _, action := range catalog.Actions {
		for _, output := range action.Outputs {
			assert.NotEmpty(t, output.Type, "action %s output %s", action.Type, output.Name)
		}
		if action.Type == "workflow.emit" {
			var fieldTypes *ct.WorkflowActionParamResponse
			for index, param := range action.Params {
				if param.Type == ct.WorkflowParamTypeFieldTypes {
					fieldTypes = &action.Params[index]
				}
			}
			require.NotNil(t, fieldTypes, "Start other workflows declares the types of its data")
			require.NotNil(t, fieldTypes.Types)
			assert.Equal(t, "data", *fieldTypes.Types)
			assert.True(t, fieldTypes.Literal)
		}
	}
}

func TestWorkflowCatalog_TypesACustomEventFromItsSender(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	upgraded := customEvent("upgraded")
	w.createWorkflow(workflowBody("organization.created", typedEmitStep("handoff", upgraded,
		`{"organization_id": "{{event.data.organization_id}}", "seats": "5", "plan": "pro"}`,
		`{"organization_id": "organization", "seats": "number"}`,
	)))

	var found *ct.WorkflowTriggerResponse
	catalog := w.catalog()
	for index, trigger := range catalog.Triggers {
		if trigger.Type == workflow.CustomEventPrefix+upgraded {
			found = &catalog.Triggers[index]
		}
	}

	require.NotNil(t, found)
	types := map[string]ct.WorkflowFieldType{}
	for _, field := range found.Fields {
		types[field.Name] = field.Type
	}
	assert.Equal(t, map[string]ct.WorkflowFieldType{
		"organization_id": ct.WorkflowFieldTypeOrganization,
		"plan":            ct.WorkflowFieldTypeText,
		"seats":           ct.WorkflowFieldTypeNumber,
	}, types)
}

func TestCreateWorkflow_RefusesASecondTypeForACustomEventField(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	upgraded := customEvent("upgraded")
	first := w.createWorkflow(workflowBody("organization.created",
		typedEmitStep("handoff", upgraded, `{"seats": "5"}`, `{"seats": "number"}`)))

	resp := w.createWorkflowRaw(workflowBody("product_user.created",
		typedEmitStep("notify", upgraded, `{"seats": "five"}`, `{"seats": "text"}`)))

	assertErrorCode(t, resp.StatusCode(), resp.Body, http.StatusBadRequest, "WORKFLOW_EVENT_FIELD_CONFLICT")
	var envelope ct.ApiErrorResponse
	require.NoError(t, json.Unmarshal(resp.Body, &envelope))
	require.NotNil(t, envelope.Errors[0].Metadata)
	metadata := *envelope.Errors[0].Metadata
	assert.Equal(t, "steps[0].params.data_types", metadata["location"])
	assert.Equal(t, "seats", metadata["field"])
	assert.Equal(t, "number", metadata["other_type"])
	assert.Equal(t, first.Id, metadata["other_workflow_id"])
}
