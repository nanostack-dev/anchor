package engine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
	"anchor/internal/workflow/engine"
)

func wf(id, trigger string, enabled bool, steps ...workflow.Step) workflow.Workflow {
	return workflow.Workflow{
		ID: id, Name: id, Enabled: enabled, TriggerEventType: trigger,
		Definition: workflow.Definition{Steps: steps},
	}
}

func createWorkspace() workflow.Step {
	return workflow.Step{ID: "ws", Action: engine.ActionWorkspaceCreate, Params: map[string]string{
		"organization_id": "{{event.data.organization_id}}", "name": "x",
	}}
}

func renameOrganization() workflow.Step {
	return workflow.Step{ID: "rename", Action: engine.ActionOrganizationUpdate, Params: map[string]string{
		"organization_id": "{{event.data.organization_id}}", "name": "x",
	}}
}

func emit(name string) workflow.Step {
	return workflow.Step{ID: "emit", Action: engine.ActionWorkflowEmit, Params: map[string]string{"event": name}}
}

func TestFindLoop_SeesAWorkflowThatStartsItself(t *testing.T) {
	path := newEngine(t).FindLoop(wf("a", string(events.WorkspaceCreated), true, createWorkspace()), nil)

	require.Len(t, path, 1)
	assert.Equal(t, string(events.WorkspaceCreated), path[0].Emits)
}

func TestFindLoop_FollowsOtherEnabledWorkflows(t *testing.T) {
	others := []workflow.Workflow{
		wf("b", string(events.OrganizationUpdated), true, createWorkspace()),
	}

	path := newEngine(t).FindLoop(wf("a", string(events.WorkspaceCreated), true, renameOrganization()), others)

	require.Len(t, path, 2)
	assert.Equal(t, []string{"a", "b"}, []string{path[0].WorkflowID, path[1].WorkflowID})
	assert.Contains(t, engine.DescribeLoop(path), "“a” (on workspace.created) emits organization.updated")
}

func TestFindLoop_IgnoresDisabledWorkflows(t *testing.T) {
	others := []workflow.Workflow{
		wf("b", string(events.OrganizationUpdated), false, createWorkspace()),
	}

	assert.Nil(t, newEngine(t).FindLoop(wf("a", string(events.WorkspaceCreated), true, renameOrganization()), others))
	assert.Nil(t, newEngine(t).FindLoop(wf("c", string(events.WorkspaceCreated), false, createWorkspace()), nil))
}

func TestFindLoop_ComparesTheCandidateWithItsNewVersionOnly(t *testing.T) {
	stored := wf("a", string(events.WorkspaceCreated), true, createWorkspace())

	edited := wf("a", string(events.WorkspaceCreated), true, renameOrganization())

	assert.Nil(t, newEngine(t).FindLoop(edited, []workflow.Workflow{stored}))
}

func TestFindLoop_FollowsCustomEvents(t *testing.T) {
	others := []workflow.Workflow{
		wf("b", "custom.pong", true, emit("ping")),
		wf("c", "custom.unrelated", true, emit("ping")),
	}

	path := newEngine(t).FindLoop(wf("a", "custom.ping", true, emit("pong")), others)

	require.Len(t, path, 2)
	assert.Equal(t, "custom.pong", path[0].Emits)
	assert.Equal(t, "custom.ping", path[1].Emits)
}

func TestFindLoop_NarrowsOrganizationCreateToItsParameters(t *testing.T) {
	withoutOwner := workflow.Step{ID: "org", Action: engine.ActionOrganizationCreate, Params: map[string]string{
		"name": "Personal",
	}}
	withOwner := workflow.Step{ID: "org", Action: engine.ActionOrganizationCreate, Params: map[string]string{
		"name": "Personal", "owner_product_user_id": "{{event.data.product_user_id}}", "owner_role_id": "r",
	}}

	assert.Nil(t, newEngine(t).FindLoop(wf("a", string(events.MembershipCreated), true, withoutOwner), nil))
	assert.NotNil(t, newEngine(t).FindLoop(wf("a", string(events.MembershipCreated), true, withOwner), nil))
}

func TestValidate_AcceptsAnyDataFieldOfACustomTrigger(t *testing.T) {
	err := newEngine(t).Validate("custom.billing.upgraded", workflow.Definition{Steps: []workflow.Step{{
		ID: "org", Action: engine.ActionOrganizationGet,
		Params: map[string]string{"organization_id": "{{event.data.whatever_the_emitter_sent}}"},
	}}})

	require.NoError(t, err)
}

func TestValidate_RejectsAMalformedCustomTrigger(t *testing.T) {
	err := newEngine(t).Validate("custom.Billing Upgraded", workflow.Definition{Steps: []workflow.Step{emit("x")}})

	assert.Contains(t, validationLocation(t, err), "not a valid custom event")
}

func TestValidate_RejectsAComputedCustomEventName(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{Steps: []workflow.Step{
		emit("org.{{event.data.organization_id}}"),
	}})

	assert.Contains(t, validationLocation(t, err), "cannot hold {{ }} references")
}

func TestValidate_RejectsAnOptionTheParameterDoesNotOffer(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{Steps: []workflow.Step{{
		ID: "call", Action: engine.ActionHTTPRequest,
		Params: map[string]string{"url": "https://example.com/hook", "method": "TRACE"},
	}}})

	assert.Contains(t, validationLocation(t, err), "accepts POST, PUT, PATCH, GET, DELETE")
}
