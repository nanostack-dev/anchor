package ct_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/domain/workflow"
	"anchor/internal/events"
)

func customEvent(prefix string) string {
	return prefix + "_" + strings.ReplaceAll(itshared.Faker.UUID().V4()[:8], "-", "")
}

func emitStep(id, name, data string) ct.WorkflowStep {
	return step(id, "workflow.emit", map[string]string{"event": name, "data": data})
}

func (w workflowWorld) createWorkflowRaw(body ct.WorkflowWriteRequest) *ct.CreateWorkflowResponse {
	w.t.Helper()
	resp, err := w.client.CreateWorkflowWithResponse(context.Background(), w.product.ProductID, body)
	require.NoError(w.t, err)
	return resp
}

func assertLoopRefused(t *testing.T, resp *ct.CreateWorkflowResponse, hops int) {
	t.Helper()
	assertErrorCode(t, resp.StatusCode(), resp.Body, http.StatusBadRequest, "WORKFLOW_LOOP")
	var envelope struct {
		Errors []struct {
			Metadata struct {
				Loop []map[string]string `json:"loop"`
			} `json:"metadata"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(resp.Body, &envelope))
	assert.Len(t, envelope.Errors[0].Metadata.Loop, hops, string(resp.Body))
}

func TestCreateWorkflow_RefusesAWorkflowThatStartsItself(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)

	resp := w.createWorkflowRaw(workflowBody("workspace.created",
		step("again", "workspace.create", map[string]string{
			"organization_id": "{{event.data.organization_id}}", "name": "copy",
		}),
	))

	assertLoopRefused(t, resp, 1)
}

func TestCreateWorkflow_RefusesALoopThroughAnotherWorkflow(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	w.createWorkflow(workflowBody("workspace.created",
		step("rename", "organization.update", map[string]string{
			"organization_id": "{{event.data.organization_id}}", "metadata": `{"touched": true}`,
		}),
	))

	resp := w.createWorkflowRaw(workflowBody("organization.updated",
		step("ws", "workspace.create", map[string]string{
			"organization_id": "{{event.data.organization_id}}", "name": "again",
		}),
	))

	assertLoopRefused(t, resp, 2)
}

func TestCreateWorkflow_RefusesALoopOfCustomEvents(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	ping, pong := customEvent("ping"), customEvent("pong")
	w.createWorkflow(workflowBody(workflow.CustomEventPrefix+ping, emitStep("pong", pong, "")))

	resp := w.createWorkflowRaw(workflowBody(workflow.CustomEventPrefix+pong, emitStep("ping", ping, "")))

	assertLoopRefused(t, resp, 2)
}

func TestUpdateWorkflow_RefusesEnablingAWorkflowThatClosesALoop(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	body := workflowBody("organization.updated", step("rename", "organization.update", map[string]string{
		"organization_id": "{{event.data.organization_id}}", "name": "renamed",
	}))
	body.Enabled = false
	created := w.createWorkflow(body)

	body.Enabled = true
	resp, err := w.client.UpdateWorkflowWithResponse(context.Background(), w.product.ProductID, created.Id, body)

	require.NoError(t, err)
	assertErrorCode(t, resp.StatusCode(), resp.Body, http.StatusBadRequest, "WORKFLOW_LOOP")
}

func TestWorkflow_CustomEventStartsAnotherWorkflow(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	started := customEvent("onboarding")
	follower := w.createWorkflow(workflowBody(workflow.CustomEventPrefix+started,
		step("ws", "workspace.create", map[string]string{
			"organization_id": "{{event.data.org}}", "name": "Kickoff for {{event.data.plan}}",
		}),
	))
	leader := w.createWorkflow(workflowBody("organization.created",
		emitStep("handoff", started, `{"org": "{{event.data.organization_id}}", "plan": "pro"}`),
	))

	organizationID := w.newOrganization()

	leaderRuns := w.waitForRuns(leader.Id, 1)
	require.Equal(t, ct.WorkflowRunStatusSucceeded, leaderRuns[0].Status, leaderRuns[0].Error)
	followerRuns := w.waitForRuns(follower.Id, 1)
	require.Equal(t, ct.WorkflowRunStatusSucceeded, followerRuns[0].Status, followerRuns[0].Error)
	assert.Equal(t, workflow.CustomEventPrefix+started, followerRuns[0].EventType)
	assert.Equal(t, map[string]string{"org": organizationID, "plan": "pro"}, followerRuns[0].EventData)
	assert.Equal(t, []string{"Kickoff for pro"}, w.workspaceNames(organizationID))
}

func TestWorkflowCatalog_ListsTheCustomEventsWorkflowsEmit(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	started := customEvent("billing")
	w.createWorkflow(workflowBody(
		"organization.created",
		emitStep(
			"handoff",
			started,
			`{"org": "{{event.data.organization_id}}", "meta": {{event.data.organization_id}}}`,
		),
	))

	resp, err := w.client.GetWorkflowCatalogWithResponse(context.Background(), w.product.ProductID)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	var found *ct.WorkflowTriggerResponse
	for index, trigger := range resp.JSON200.Triggers {
		if trigger.Type == workflow.CustomEventPrefix+started {
			found = &resp.JSON200.Triggers[index]
		}
	}
	require.NotNil(t, found, "the emitted custom event is offered as a trigger")
	assert.Equal(t, ct.ProductEventGroupType("custom"), found.GroupType)
	assert.Equal(t, []string{"meta", "org"}, found.DataFields)
}

// The first workflow and the hops make MaxCausationDepth chained runs; the
// tail would be one more.
func TestWorkflow_ChainStopsAtTheMaximumDepth(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	names := make([]string, workflow.MaxCausationDepth)
	for index := range names {
		names[index] = customEvent("hop")
	}
	var lastHop ct.WorkflowResponse
	for index := len(names) - 2; index >= 0; index-- {
		lastHop = w.createWorkflow(workflowBody(workflow.CustomEventPrefix+names[index],
			emitStep("next", names[index+1], "")))
	}
	tail := w.createWorkflow(workflowBody(workflow.CustomEventPrefix+names[len(names)-1],
		step("org", "organization.get", map[string]string{"organization_id": "org_none"})))
	w.createWorkflow(workflowBody("organization.created", emitStep("first", names[0], "")))

	w.newOrganization()

	require.Equal(t, ct.WorkflowRunStatusSucceeded, w.waitForRuns(lastHop.Id, 1)[0].Status)
	stopped := w.waitForRuns(tail.Id, 1)[0]
	assert.Equal(t, ct.WorkflowRunStatusSkipped, stopped.Status)
	require.NotNil(t, stopped.Error)
	assert.Contains(t, *stopped.Error, "Chain stopped")
}

type backendStub struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []*http.Request
	bodies   [][]byte
	respond  func(request *http.Request) (int, string)
}

func newBackendStub(t *testing.T, respond func(request *http.Request) (int, string)) *backendStub {
	t.Helper()
	stub := &backendStub{respond: respond}
	stub.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		stub.mu.Lock()
		stub.requests = append(stub.requests, request)
		stub.bodies = append(stub.bodies, body)
		stub.mu.Unlock()
		status, answer := stub.respond(request)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(answer))
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func (b *backendStub) first() (*http.Request, []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.requests[0], b.bodies[0]
}

func TestWorkflow_CallYourBackendGivesItsAnswerToLaterSteps(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	sink := w.product.CaptureFilteredEvents([]string{})
	backend := newBackendStub(t, func(*http.Request) (int, string) {
		return http.StatusOK, `{"plan": "enterprise", "seats": 25}`
	})
	created := w.createWorkflow(workflowBody("organization.created",
		step("call", "http.request", map[string]string{"url": backend.server.URL + "/hooks/anchor"}),
		step("tag", "organization.update", map[string]string{
			"organization_id": "{{event.data.organization_id}}",
			"metadata":        `{"plan": "{{steps.call.body.plan}}"}`,
		}),
	))

	organizationID := w.newOrganization()

	run := w.waitForRuns(created.Id, 1)[0]
	require.Equal(t, ct.WorkflowRunStatusSucceeded, run.Status, run.Error)
	request, body := backend.first()
	assert.Equal(t, http.MethodPost, request.Method)
	assert.True(t, itdsl.StandardWebhookSignatureMatches(sink.Secret, request.Header, body),
		"the call is signed with the product's event signing secret")
	causation, ok := events.DecodeCausationHeader(request.Header.Get(events.CausationHeader))
	require.True(t, ok)
	assert.Equal(t, []string{created.Id}, causation.WorkflowIDs)
	assert.Contains(t, string(body), organizationID)

	org, err := w.client.GetProductOrganizationWithResponse(
		context.Background(), w.product.ProductID, organizationID, &ct.GetProductOrganizationParams{},
	)
	require.NoError(t, err)
	require.NotNil(t, org.JSON200.Metadata)
	assert.Equal(t, "enterprise", (*org.JSON200.Metadata)["plan"])
}

func TestWorkflow_CallYourBackendNeedsTheEventEndpointSecret(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	backend := newBackendStub(t, func(*http.Request) (int, string) { return http.StatusOK, `{}` })
	created := w.createWorkflow(workflowBody("organization.created",
		step("call", "http.request", map[string]string{"url": backend.server.URL}),
	))

	w.newOrganization()

	run := w.waitForRuns(created.Id, 1)[0]
	require.Equal(t, ct.WorkflowRunStatusFailed, run.Status)
	assert.Contains(t, *run.Steps[0].Error, "event endpoint")
}

// A backend that writes to Anchor while answering a workflow step, and sends
// the causation header back, cannot restart the workflow that called it.
func TestWorkflow_LoopThroughYourBackendIsPrevented(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	w.product.CaptureFilteredEvents([]string{})
	backend := newBackendStub(t, func(request *http.Request) (int, string) {
		resp, err := w.client.CreateProductOrganizationWithResponse(
			context.Background(), w.product.ProductID,
			ct.CreateProductOrganizationJSONRequestBody{Name: itdsl.UniqueOrganizationName()},
			func(_ context.Context, outgoing *http.Request) error {
				outgoing.Header.Set(events.CausationHeader, request.Header.Get(events.CausationHeader))
				return nil
			},
		)
		if err != nil || resp.StatusCode() != http.StatusCreated {
			return http.StatusInternalServerError, `{}`
		}
		return http.StatusOK, `{"organization_id": "` + resp.JSON201.Id + `"}`
	})
	created := w.createWorkflow(workflowBody("organization.created",
		step("call", "http.request", map[string]string{"url": backend.server.URL}),
	))

	w.newOrganization()

	runs := w.waitForRuns(created.Id, 2)
	byStatus := map[ct.WorkflowRunStatus]ct.WorkflowRunResponse{}
	for _, run := range runs {
		byStatus[run.Status] = run
	}
	require.Contains(t, byStatus, ct.WorkflowRunStatusSucceeded)
	prevented, ok := byStatus[ct.WorkflowRunStatusSkipped]
	require.True(t, ok, "the organization the backend created is refused by the loop guard")
	require.NotNil(t, prevented.Error)
	assert.Contains(t, *prevented.Error, "Loop prevented")
	assert.Len(t, w.runs(created.Id), 2)
}

func TestRunWorkflow_RefusesARequestFromItsOwnRun(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	created := w.createWorkflow(workflowBody("organization.created", generalWorkspaceStep()))
	header := events.EncodeCausationHeader(events.Causation{Depth: 1, WorkflowIDs: []string{created.Id}})

	resp, err := w.client.RunWorkflowWithResponse(context.Background(), w.product.ProductID, created.Id,
		ct.RunWorkflowJSONRequestBody{EventData: map[string]string{"organization_id": w.organizationID}},
		func(_ context.Context, outgoing *http.Request) error {
			outgoing.Header.Set(events.CausationHeader, header)
			return nil
		},
	)

	require.NoError(t, err)
	assertErrorCode(t, resp.StatusCode(), resp.Body, http.StatusConflict, "WORKFLOW_LOOP")
}

func TestListWorkflows_IncludesTheLastRun(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)
	ran := w.createWorkflow(workflowBody("organization.created", generalWorkspaceStep()))
	idle := w.createWorkflow(workflowBody("workspace.deleted", step("org", "organization.get",
		map[string]string{"organization_id": "{{event.data.organization_id}}"})))
	w.newOrganization()
	w.waitForRuns(ran.Id, 1)

	include := []ct.WorkflowInclude{ct.LastRun}
	resp, err := w.client.ListWorkflowsWithResponse(context.Background(), w.product.ProductID,
		&ct.ListWorkflowsParams{Include: &include})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	byID := map[string]ct.WorkflowResponse{}
	for _, item := range resp.JSON200.Items {
		byID[item.Id] = item
	}
	require.NotNil(t, byID[ran.Id].LastRun)
	assert.Equal(t, ct.WorkflowRunStatusSucceeded, byID[ran.Id].LastRun.Status)
	assert.Nil(t, byID[idle.Id].LastRun)
	assert.Equal(t, []string{"workspace.created"}, byID[ran.Id].Emits)
}

func TestCreateWorkflow_RefusesAComputedCustomEventName(t *testing.T) {
	t.Parallel()
	w := newWorkflowWorld(t)

	resp := w.createWorkflowRaw(workflowBody("organization.created",
		emitStep("handoff", "org.{{event.data.organization_id}}", "")))

	assertErrorCode(t, resp.StatusCode(), resp.Body, http.StatusBadRequest, "INVALID_WORKFLOW_DEFINITION")
}
