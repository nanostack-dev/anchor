package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generalWorkspaceStep() ct.WorkflowStep {
	return step("ws", "workspace.create", map[string]string{
		"organization_id": "{{event.data.organization_id}}",
		"name":            "General",
	})
}

func TestCreateWorkflow_ReadsBackItsDefinition(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	body := workflowBody("organization.created", generalWorkspaceStep())
	body.Definition.Conditions = []ct.WorkflowCondition{
		{Field: "event.data.organization_id", Operator: ct.Exists},
	}

	created := w.createWorkflow(body)

	resp, err := w.client.GetWorkflowWithResponse(context.Background(), w.product.ProductID, created.Id)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, body.Name, resp.JSON200.Name)
	assert.Equal(t, "organization.created", resp.JSON200.TriggerEventType)
	assert.Equal(t, body.Definition.Conditions, resp.JSON200.Definition.Conditions)
	assert.Equal(t, body.Definition.Steps, resp.JSON200.Definition.Steps)
}

func TestListWorkflows_ReturnsTheProductsWorkflows(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	first := w.createWorkflow(workflowBody("organization.created", generalWorkspaceStep()))
	second := w.createWorkflow(workflowBody("organization.created", generalWorkspaceStep()))

	resp, err := w.client.ListWorkflowsWithResponse(context.Background(), w.product.ProductID)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	ids := []string{}
	for _, item := range resp.JSON200.Items {
		ids = append(ids, item.Id)
	}
	assert.ElementsMatch(t, []string{first.Id, second.Id}, ids)
}

func TestUpdateWorkflow_ReplacesTheDefinition(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	created := w.createWorkflow(workflowBody("organization.created", generalWorkspaceStep()))
	replacement := workflowBody("workspace.created", step("org", "organization.get", map[string]string{
		"organization_id": "{{event.data.organization_id}}",
	}))
	replacement.Enabled = false

	resp, err := w.client.UpdateWorkflowWithResponse(context.Background(), w.product.ProductID, created.Id, replacement)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, "workspace.created", resp.JSON200.TriggerEventType)
	assert.False(t, resp.JSON200.Enabled)
	assert.Equal(t, replacement.Definition.Steps, resp.JSON200.Definition.Steps)
}

func TestDeleteWorkflow_RemovesIt(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	created := w.createWorkflow(workflowBody("organization.created", generalWorkspaceStep()))

	deleteResp, err := w.client.DeleteWorkflowWithResponse(context.Background(), w.product.ProductID, created.Id)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, deleteResp.StatusCode(), string(deleteResp.Body))

	getResp, err := w.client.GetWorkflowWithResponse(context.Background(), w.product.ProductID, created.Id)
	require.NoError(t, err)
	assertErrorCode(t, getResp.StatusCode(), getResp.Body, http.StatusNotFound, "WORKFLOW_NOT_FOUND")
}

func TestCreateWorkflow_RejectsAnInvalidDefinition(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	cases := map[string]ct.WorkflowWriteRequest{
		"unknown trigger": workflowBody("organization.exploded", generalWorkspaceStep()),
		"unknown action": workflowBody(
			"organization.created",
			step("x", "organization.explode", map[string]string{}),
		),
		"missing required": workflowBody("organization.created", step("ws", "workspace.create", map[string]string{})),
		"field the trigger does not carry": workflowBody("organization.created", step("u", "product_user.get",
			map[string]string{"product_user_id": "{{event.data.product_user_id}}"})),
		"reference to a later step": workflowBody("organization.created",
			step("ws", "workspace.create", map[string]string{
				"organization_id": "{{event.data.organization_id}}", "name": "{{steps.org.name}}",
			}),
			step("org", "organization.get", map[string]string{"organization_id": "{{event.data.organization_id}}"}),
		),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			resp, err := w.client.CreateWorkflowWithResponse(context.Background(), w.product.ProductID, body)
			require.NoError(t, err)
			assertErrorCode(t, resp.StatusCode(), resp.Body, http.StatusBadRequest, "INVALID_WORKFLOW_DEFINITION")
		})
	}
}

func TestGetWorkflow_IsNotFoundFromAnotherProduct(t *testing.T) {
	t.Parallel()
	owner := newWorkflowWorld(t)
	other := newWorkflowWorld(t)
	created := owner.createWorkflow(workflowBody("organization.created", generalWorkspaceStep()))

	resp, err := other.client.GetWorkflowWithResponse(context.Background(), other.product.ProductID, created.Id)

	require.NoError(t, err)
	assertErrorCode(t, resp.StatusCode(), resp.Body, http.StatusNotFound, "WORKFLOW_NOT_FOUND")
}

func TestListWorkflows_NeedsTheWorkflowReadScope(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	client, _ := w.product.CreateAPIKeyClientWithScopes([]string{"organization:read"})

	resp, err := client.ListWorkflowsWithResponse(context.Background(), w.product.ProductID)

	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode(), string(resp.Body))
}

func TestWorkflowCatalog_DescribesEveryTriggerAndAction(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)

	resp, err := w.client.GetWorkflowCatalogWithResponse(context.Background(), w.product.ProductID)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	for _, trigger := range resp.JSON200.Triggers {
		assert.NotEmpty(t, trigger.DataFields, "trigger %s declares no data field", trigger.Type)
	}
	actions := []string{}
	for _, action := range resp.JSON200.Actions {
		actions = append(actions, action.Type)
	}
	assert.Subset(t, actions, []string{
		"organization.get", "organization.create", "organization.update", "workspace.create",
		"member.add", "member.update_role", "member.remove", "invitation.create", "product_user.get",
		"license.instantiate", "license.migrate", "license.adjust", "email.send",
	})
	assert.Contains(t, resp.JSON200.Operators, ct.EndsWith)
}
