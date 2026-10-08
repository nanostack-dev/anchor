package engine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
	"anchor/internal/workflow/engine"
)

func sendTyped(id, event, data, types string) workflow.Step {
	params := map[string]string{"event": event, "data": data}
	if types != "" {
		params["data_types"] = types
	}
	return workflow.Step{ID: id, Action: engine.ActionWorkflowEmit, Params: params}
}

func triggerOf(t *testing.T, triggers []engine.Trigger, eventType string) engine.Trigger {
	t.Helper()
	for _, trigger := range triggers {
		if string(trigger.Type) == eventType {
			return trigger
		}
	}
	require.Failf(t, "trigger missing", "no trigger %s", eventType)
	return engine.Trigger{}
}

func TestTriggers_TypeTheFieldsOfACatalogEvent(t *testing.T) {
	trigger := triggerOf(t, newEngine(t).Triggers(nil), string(events.MembershipCreated))

	assert.Equal(t, []engine.FieldSpec{
		{Name: "organization_id", Type: engine.FieldOrganization, Description: "Identifier of the organization."},
		{Name: "product_user_id", Type: engine.FieldProductUser, Description: "Identifier of the product user."},
	}, trigger.Fields)
	assert.Equal(t, []string{"organization_id", "product_user_id"}, trigger.DataFields)
}

func TestTriggers_TypeACustomEventFromTheDataTypesOfItsSender(t *testing.T) {
	sender := wf("wf_a", string(events.OrganizationCreated), true, sendTyped("send", "onboarding.started",
		`{"organization_id": "{{event.data.organization_id}}", "plan": "pro"}`,
		`{"organization_id": "organization"}`))

	trigger := triggerOf(t, newEngine(t).Triggers([]workflow.Workflow{sender}), "custom.onboarding.started")

	require.Len(t, trigger.Fields, 2)
	assert.Equal(t, engine.FieldOrganization, trigger.Fields[0].Type)
	assert.Equal(t, "plan", trigger.Fields[1].Name)
	assert.Equal(t, engine.FieldText, trigger.Fields[1].Type, "a key without a declared type is text")
}

func TestActions_EveryOutputHasAFieldType(t *testing.T) {
	for _, spec := range newEngine(t).Actions() {
		for _, output := range spec.Outputs {
			assert.Truef(t, output.Type.Valid(), "%s output %s has type %q", spec.Type, output.Name, output.Type)
		}
	}
}

func TestValidate_RejectsAnUnknownFieldType(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{Steps: []workflow.Step{
		sendTyped("send", "onboarding.started", `{"plan": "pro"}`, `{"plan": "currency"}`),
	}})

	assert.Contains(t, validationLocation(t, err), `"currency" is not a field type for "plan"`)
}

func TestValidate_RejectsATypeForAKeyTheDataDoesNotHold(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{Steps: []workflow.Step{
		sendTyped("send", "onboarding.started", `{"plan": "pro"}`, `{"seats": "number"}`),
	}})

	assert.Contains(t, validationLocation(t, err), `"seats" is typed but the event data has no such key`)
}

func TestValidate_RejectsAReferenceInFieldTypes(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{Steps: []workflow.Step{
		sendTyped("send", "onboarding.started", `{"plan": "pro"}`, `{"plan": "{{event.type}}"}`),
	}})

	assert.Contains(t, validationLocation(t, err), "cannot hold {{ }} references")
}

func TestFieldTypeConflict_SeesAnotherWorkflowSendingADifferentType(t *testing.T) {
	other := wf("wf_other", string(events.OrganizationCreated), false,
		sendTyped("send", "billing.upgraded", `{"seats": "5"}`, `{"seats": "number"}`))
	candidate := wf("wf_new", string(events.ProductUserCreated), true,
		createWorkspace(), sendTyped("notify", "billing.upgraded", `{"seats": "five"}`, ""))

	conflict := newEngine(t).FieldTypeConflict(candidate, []workflow.Workflow{other})

	require.NotNil(t, conflict)
	assert.Equal(t, "custom.billing.upgraded", conflict.Event)
	assert.Equal(t, "seats", conflict.Field)
	assert.Equal(t, engine.FieldText, conflict.Type)
	assert.Equal(t, engine.FieldNumber, conflict.OtherType)
	assert.Equal(t, 1, conflict.StepIndex)
	assert.Equal(t, "wf_other", conflict.OtherID)
	assert.Contains(t, conflict.Describe(), "“wf_other” sends it as number")
}

func TestFieldTypeConflict_SeesTwoStepsOfTheSameWorkflow(t *testing.T) {
	candidate := wf("wf_new", string(events.OrganizationCreated), true,
		sendTyped("first", "billing.upgraded", `{"seats": "5"}`, `{"seats": "number"}`),
		sendTyped("second", "billing.upgraded", `{"seats": "5"}`, `{"seats": "text"}`))

	conflict := newEngine(t).FieldTypeConflict(candidate, nil)

	require.NotNil(t, conflict)
	assert.Equal(t, 1, conflict.StepIndex)
	assert.Contains(t, conflict.Describe(), `step "first" of this workflow sends it as number`)
}

func TestFieldTypeConflict_ComparesTheCandidateWithItsNewVersionOnly(t *testing.T) {
	saved := wf("wf_same", string(events.OrganizationCreated), true,
		sendTyped("send", "billing.upgraded", `{"seats": "5"}`, `{"seats": "number"}`))
	edited := wf("wf_same", string(events.OrganizationCreated), true,
		sendTyped("send", "billing.upgraded", `{"seats": "5"}`, `{"seats": "text"}`))

	assert.Nil(t, newEngine(t).FieldTypeConflict(edited, []workflow.Workflow{saved}))
}

func TestFieldTypeConflict_TreatsAnUntypedKeyAsText(t *testing.T) {
	other := wf("wf_other", string(events.OrganizationCreated), true,
		sendTyped("send", "billing.upgraded", `{"plan": "pro"}`, `{"plan": "text"}`))
	candidate := wf("wf_new", string(events.OrganizationCreated), true,
		sendTyped("send", "billing.upgraded", `{"plan": "team"}`, ""))

	assert.Nil(t, newEngine(t).FieldTypeConflict(candidate, []workflow.Workflow{other}))
}
