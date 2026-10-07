package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

func TestWorkflow_RunsItsStepsWhenTheTriggerEventHappens(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	created := w.createWorkflow(workflowBody("organization.created",
		generalWorkspaceStep(),
		step("tag", "organization.update", map[string]string{
			"organization_id": "{{event.data.organization_id}}",
			"metadata":        `{"default_workspace": "{{steps.ws.workspace_id}}", "onboarded": true}`,
		}),
	))

	organizationID := w.newOrganization()

	runs := w.waitForRuns(created.Id, 1)
	require.Len(t, runs, 1)
	run := runs[0]
	require.Equal(t, ct.WorkflowRunStatusSucceeded, run.Status, run.Error)
	assert.Equal(t, ct.Event, run.Trigger)
	assert.Equal(t, map[string]string{"organization_id": organizationID}, run.EventData)
	assert.Equal(t, []string{"General"}, w.workspaceNames(organizationID))

	org, err := w.client.GetProductOrganizationWithResponse(
		context.Background(), w.product.ProductID, organizationID, &ct.GetProductOrganizationParams{},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, org.StatusCode(), string(org.Body))
	require.NotNil(t, org.JSON200.Metadata)
	workspaceID := (*run.Steps[0].Output)["workspace_id"]
	assert.Equal(t, workspaceID, (*org.JSON200.Metadata)["default_workspace"])
	assert.Equal(t, true, (*org.JSON200.Metadata)["onboarded"])
}

func TestWorkflow_StepConditionGatesTheWrite(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	domain := "acme-" + itshared.Faker.UUID().V4()[:8] + ".test"
	join := step("join", "member.add", map[string]string{
		"organization_id": w.organizationID,
		"product_user_id": "{{steps.user.product_user_id}}",
		"role_id":         w.roleID,
	})
	join.When = &[]ct.WorkflowCondition{
		{Field: "steps.user.email_domain", Operator: ct.Equals, Value: new(domain)},
	}
	created := w.createWorkflow(workflowBody("product_user.created",
		step("user", "product_user.get", map[string]string{"product_user_id": "{{event.data.product_user_id}}"}),
		join,
	))

	insider := w.createProductUser("ada@" + domain)
	outsider := w.createProductUser("bob-" + itshared.Faker.UUID().V4()[:8] + "@elsewhere.test")

	runs := w.waitForRuns(created.Id, 2)
	byUser := map[string]ct.WorkflowRunResponse{}
	for _, run := range runs {
		byUser[run.EventData["product_user_id"]] = run
	}
	require.Equal(t, ct.WorkflowRunStatusSucceeded, byUser[insider].Status, byUser[insider].Error)
	assert.Equal(t, ct.WorkflowStepStatusSucceeded, byUser[insider].Steps[1].Status)
	assert.Equal(t, ct.WorkflowStepStatusSkipped, byUser[outsider].Steps[1].Status)
	assert.Equal(t, http.StatusOK, w.memberStatus(w.organizationID, insider))
	assert.Equal(t, http.StatusNotFound, w.memberStatus(w.organizationID, outsider))
}

func TestWorkflow_ConditionsThatDoNotHoldSkipTheRun(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	body := workflowBody("organization.created", generalWorkspaceStep())
	body.Definition.Conditions = []ct.WorkflowCondition{
		{Field: "event.data.organization_id", Operator: ct.Equals, Value: new("org_none")},
	}
	created := w.createWorkflow(body)

	organizationID := w.newOrganization()

	runs := w.waitForRuns(created.Id, 1)
	assert.Equal(t, ct.WorkflowRunStatusSkipped, runs[0].Status)
	assert.Empty(t, w.workspaceNames(organizationID))
}

func TestWorkflow_DisabledStartsNoRun(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	disabledBody := workflowBody("organization.created", generalWorkspaceStep())
	disabledBody.Enabled = false
	disabled := w.createWorkflow(disabledBody)
	enabled := w.createWorkflow(workflowBody("organization.created", step("org", "organization.get",
		map[string]string{"organization_id": "{{event.data.organization_id}}"})))

	w.newOrganization()

	w.waitForRuns(enabled.Id, 1)
	assert.Empty(t, w.runs(disabled.Id))
}

func TestWorkflow_AFailedStepFailsTheRunAndStopsIt(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	created := w.createWorkflow(workflowBody("organization.created",
		step("join", "member.add", map[string]string{
			"organization_id": "{{event.data.organization_id}}",
			"product_user_id": "pusr_doesnotexist",
			"role_id":         w.roleID,
		}),
		generalWorkspaceStep(),
	))

	organizationID := w.newOrganization()

	runs := w.waitForRuns(created.Id, 1)
	require.Equal(t, ct.WorkflowRunStatusFailed, runs[0].Status)
	require.Len(t, runs[0].Steps, 1)
	assert.Equal(t, ct.WorkflowStepStatusFailed, runs[0].Steps[0].Status)
	assert.NotEmpty(t, runs[0].Steps[0].Error)
	assert.Empty(t, w.workspaceNames(organizationID))
}

func TestRunWorkflow_RunsNowAgainstTheGivenEventData(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	body := workflowBody("organization.created", generalWorkspaceStep())
	body.Enabled = false
	created := w.createWorkflow(body)

	resp, err := w.client.RunWorkflowWithResponse(context.Background(), w.product.ProductID, created.Id,
		ct.RunWorkflowJSONRequestBody{EventData: map[string]string{"organization_id": w.organizationID}})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, ct.WorkflowRunStatusSucceeded, resp.JSON200.Status, resp.JSON200.Error)
	assert.Equal(t, ct.Manual, resp.JSON200.Trigger)
	assert.Equal(t, []string{"General"}, w.workspaceNames(w.organizationID))
	runs := w.runs(created.Id)
	require.Len(t, runs, 1)
	assert.Equal(t, resp.JSON200.Id, runs[0].Id)
}

func TestDryRunWorkflow_ReadsButWritesNothing(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	body := workflowBody("organization.created",
		step("org", "organization.get", map[string]string{"organization_id": "{{event.data.organization_id}}"}),
		step("ws", "workspace.create", map[string]string{
			"organization_id": "{{event.data.organization_id}}",
			"name":            "{{steps.org.name}} team",
		}),
	)

	resp, err := w.client.DryRunWorkflowWithResponse(context.Background(), w.product.ProductID,
		ct.DryRunWorkflowJSONRequestBody{
			Workflow:  body,
			EventData: map[string]string{"organization_id": w.organizationID},
		})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	run := resp.JSON200
	require.Equal(t, ct.WorkflowRunStatusSucceeded, run.Status, run.Error)
	assert.Equal(t, ct.DryRun, run.Trigger)
	assert.Equal(t, ct.WorkflowStepStatusSucceeded, run.Steps[0].Status)
	assert.Equal(t, ct.WorkflowStepStatusSimulated, run.Steps[1].Status)
	orgName := (*run.Steps[0].Output)["name"]
	assert.Equal(t, orgName.(string)+" team", (*run.Steps[1].Params)["name"])
	assert.Empty(t, w.workspaceNames(w.organizationID))
}

func TestListProductWorkflowRuns_NamesTheWorkflow(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	created := w.createWorkflow(workflowBody("organization.created", generalWorkspaceStep()))
	w.newOrganization()
	w.waitForRuns(created.Id, 1)

	resp, err := w.client.ListProductWorkflowRunsWithResponse(
		context.Background(), w.product.ProductID, &ct.ListProductWorkflowRunsParams{},
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	require.Len(t, resp.JSON200.Items, 1)
	assert.Equal(t, created.Name, *resp.JSON200.Items[0].WorkflowName)
}
