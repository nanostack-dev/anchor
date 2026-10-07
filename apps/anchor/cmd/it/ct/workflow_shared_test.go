package ct_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

const (
	workflowWaitTimeout = 30 * time.Second
	workflowWaitTick    = 100 * time.Millisecond
)

type workflowWorld struct {
	world
	client *ct.ClientWithResponses
}

func newWorkflowWorld(t *testing.T) workflowWorld {
	t.Helper()
	w := newWorld(t)
	return workflowWorld{world: w, client: w.product.AllScopeAPIKeyClient()}
}

func step(id, action string, params map[string]string) ct.WorkflowStep {
	return ct.WorkflowStep{Id: id, Action: action, Params: params}
}

func workflowBody(trigger string, steps ...ct.WorkflowStep) ct.WorkflowWriteRequest {
	return ct.WorkflowWriteRequest{
		Name:             "workflow-" + itshared.Faker.UUID().V4()[:8],
		Enabled:          true,
		TriggerEventType: trigger,
		Definition:       ct.WorkflowDefinition{Conditions: []ct.WorkflowCondition{}, Steps: steps},
	}
}

func (w workflowWorld) createWorkflow(body ct.WorkflowWriteRequest) ct.WorkflowResponse {
	w.t.Helper()
	resp, err := w.client.CreateWorkflowWithResponse(context.Background(), w.product.ProductID, body)
	require.NoError(w.t, err)
	require.Equal(w.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return *resp.JSON201
}

func (w workflowWorld) runs(workflowID string) []ct.WorkflowRunResponse {
	w.t.Helper()
	resp, err := w.client.ListWorkflowRunsWithResponse(
		context.Background(), w.product.ProductID, workflowID, &ct.ListWorkflowRunsParams{},
	)
	require.NoError(w.t, err)
	require.Equal(w.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	return resp.JSON200.Items
}

func (w workflowWorld) waitForRuns(workflowID string, count int) []ct.WorkflowRunResponse {
	w.t.Helper()
	var runs []ct.WorkflowRunResponse
	require.Eventually(w.t, func() bool {
		runs = w.runs(workflowID)
		finished := 0
		for _, run := range runs {
			if run.Status != ct.WorkflowRunStatusRunning {
				finished++
			}
		}
		return finished >= count
	}, workflowWaitTimeout, workflowWaitTick, "workflow %s never finished %d runs", workflowID, count)
	return runs
}

func (w workflowWorld) workspaceNames(organizationID string) []string {
	w.t.Helper()
	resp, err := w.client.SearchOrganizationWorkspacesWithResponse(
		context.Background(), w.product.ProductID, organizationID,
		ct.SearchOrganizationWorkspacesJSONRequestBody{
			Pagination: &ct.PaginationRequest{Limit: new(int32(100)), Offset: new(int32(0))},
		},
	)
	require.NoError(w.t, err)
	require.Equal(w.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	names := make([]string, 0, len(resp.JSON200.Items))
	for _, item := range resp.JSON200.Items {
		names = append(names, item.Name)
	}
	return names
}

func (w workflowWorld) createProductUser(email string) string {
	w.t.Helper()
	resp, err := w.client.CreateProductUserWithResponse(
		context.Background(),
		w.product.ProductID,
		ct.CreateProductUserJSONRequestBody{Email: email, Name: new("Workflow user")},
	)
	require.NoError(w.t, err)
	require.Equal(w.t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return resp.JSON201.Id
}

func (w workflowWorld) memberStatus(organizationID, productUserID string) int {
	w.t.Helper()
	resp, err := w.client.GetOrganizationMemberWithResponse(
		context.Background(), w.product.ProductID, organizationID, productUserID, &ct.GetOrganizationMemberParams{},
	)
	require.NoError(w.t, err)
	return resp.StatusCode()
}

func assertErrorCode(t *testing.T, status int, body []byte, wantStatus int, wantCode string) {
	t.Helper()
	require.Equal(t, wantStatus, status, string(body))
	var envelope struct {
		Errors []ct.ApiError `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope), string(body))
	assert.Equal(t, wantCode, errorCode(t, envelope.Errors), string(body))
}
