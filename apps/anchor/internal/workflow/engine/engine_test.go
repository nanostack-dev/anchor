package engine_test

import (
	"context"
	"testing"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
	"anchor/internal/workflow/engine"
)

func newEngine(t *testing.T) *engine.Engine {
	t.Helper()
	catalog := events.NewCatalog(events.CatalogParams{Registrations: []events.Registration{
		events.RegisterInternal(events.GroupOrganizations,
			events.Definition{Type: events.OrganizationCreated, Name: "Organization created"},
			events.Definition{Type: events.MembershipCreated, Name: "Member added"},
		),
		events.RegisterInternal(events.GroupUsers,
			events.Definition{Type: events.ProductUserCreated, Name: "Product user created"},
		),
	}})
	return engine.New(engine.Services{}, catalog)
}

func validationLocation(t *testing.T, err error) string {
	t.Helper()
	require.Error(t, err)
	faultErr, ok := fault.As(err)
	require.True(t, ok, "expected a fault, got %v", err)
	return faultErr.Error()
}

func TestRender_KeepsTheTypeOfAWholeReference(t *testing.T) {
	scope := engine.NewScope(
		"wf_1",
		"Demo",
		"evt_1",
		"organization.created",
		map[string]string{"organization_id": "org_1"},
	)
	scope.SetStepOutput("org", map[string]any{"metadata": map[string]any{"plan": "pro", "seats": 5.0}})

	whole, err := scope.Render("{{ steps.org.metadata }}")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"plan": "pro", "seats": 5.0}, whole)

	text, err := scope.Render(
		"Org {{event.data.organization_id}} on {{steps.org.metadata.plan}} with {{steps.org.metadata.seats}}",
	)
	require.NoError(t, err)
	assert.Equal(t, "Org org_1 on pro with 5", text)
}

func TestRender_FailsOnAReferenceWithNoValue(t *testing.T) {
	scope := engine.NewScope("wf_1", "Demo", "evt_1", "organization.created", map[string]string{})

	_, err := scope.Render("{{event.data.organization_id}}")

	require.ErrorContains(t, err, "event.data.organization_id")
}

func TestHolds_ComparesWithoutLetterCase(t *testing.T) {
	scope := engine.NewScope("wf_1", "Demo", "evt_1", "product_user.created", map[string]string{})
	scope.SetStepOutput("user", map[string]any{"email": "Ada@Acme.COM", "email_domain": "acme.com", "name": ""})

	cases := []struct {
		name      string
		condition workflow.Condition
		want      bool
	}{
		{
			"equals",
			workflow.Condition{Field: "steps.user.email_domain", Operator: workflow.OperatorEquals, Value: "ACME.com"},
			true,
		},
		{
			"not equals",
			workflow.Condition{
				Field:    "steps.user.email_domain",
				Operator: workflow.OperatorNotEquals,
				Value:    "acme.com",
			},
			false,
		},
		{
			"ends with",
			workflow.Condition{Field: "steps.user.email", Operator: workflow.OperatorEndsWith, Value: "@acme.com"},
			true,
		},
		{
			"starts with",
			workflow.Condition{Field: "steps.user.email", Operator: workflow.OperatorStartsWith, Value: "ada@"},
			true,
		},
		{
			"contains",
			workflow.Condition{Field: "steps.user.email", Operator: workflow.OperatorContains, Value: "acme"},
			true,
		},
		{
			"not contains",
			workflow.Condition{Field: "steps.user.email", Operator: workflow.OperatorNotContains, Value: "acme"},
			false,
		},
		{
			"in",
			workflow.Condition{
				Field:    "steps.user.email_domain",
				Operator: workflow.OperatorIn,
				Value:    "globex.com, acme.com",
			},
			true,
		},
		{"exists", workflow.Condition{Field: "steps.user.email", Operator: workflow.OperatorExists}, true},
		{
			"empty string is absent",
			workflow.Condition{Field: "steps.user.name", Operator: workflow.OperatorNotExists},
			true,
		},
		{
			"missing path is absent",
			workflow.Condition{Field: "steps.user.phone", Operator: workflow.OperatorExists},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := scope.Holds([]workflow.Condition{tc.condition})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestValidate_AcceptsAChainThatReadsEarlierSteps(t *testing.T) {
	eng := newEngine(t)

	err := eng.Validate(string(events.ProductUserCreated), workflow.Definition{
		Steps: []workflow.Step{
			{ID: "user", Action: engine.ActionProductUserGet, Params: map[string]string{
				"product_user_id": "{{event.data.product_user_id}}",
			}},
			{
				ID:     "join",
				Action: engine.ActionMemberAdd,
				Params: map[string]string{
					"organization_id": "org_1",
					"product_user_id": "{{steps.user.product_user_id}}",
					"role_id":         "role_1",
				},
				When: []workflow.Condition{
					{Field: "steps.user.email_domain", Operator: workflow.OperatorEquals, Value: "acme.com"},
				},
			},
		},
	})

	require.NoError(t, err)
}

func TestValidate_RejectsAnUnknownTrigger(t *testing.T) {
	err := newEngine(t).Validate("organization.exploded", workflow.Definition{
		Steps: []workflow.Step{
			{ID: "a", Action: engine.ActionOrganizationGet, Params: map[string]string{"organization_id": "x"}},
		},
	})
	assert.Contains(t, validationLocation(t, err), "organization.exploded")
}

func TestValidate_RejectsAWorkflowWithNoStep(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{})
	assert.Contains(t, validationLocation(t, err), "at least one step")
}

func TestValidate_RejectsAMissingRequiredParameter(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{
		Steps: []workflow.Step{{ID: "ws", Action: engine.ActionWorkspaceCreate, Params: map[string]string{
			"organization_id": "{{event.data.organization_id}}",
		}}},
	})
	assert.Contains(t, validationLocation(t, err), `"name"`)
}

func TestValidate_RejectsAnUndeclaredParameter(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{
		Steps: []workflow.Step{{ID: "org", Action: engine.ActionOrganizationGet, Params: map[string]string{
			"organization_id": "{{event.data.organization_id}}", "colour": "red",
		}}},
	})
	assert.Contains(t, validationLocation(t, err), "colour")
}

func TestValidate_RejectsADataFieldTheTriggerDoesNotCarry(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{
		Steps: []workflow.Step{{ID: "user", Action: engine.ActionProductUserGet, Params: map[string]string{
			"product_user_id": "{{event.data.product_user_id}}",
		}}},
	})
	assert.Contains(t, validationLocation(t, err), "product_user_id")
}

func TestValidate_RejectsAReferenceToALaterStep(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{
		Steps: []workflow.Step{
			{ID: "ws", Action: engine.ActionWorkspaceCreate, Params: map[string]string{
				"organization_id": "{{event.data.organization_id}}", "name": "{{steps.org.name}}",
			}},
			{ID: "org", Action: engine.ActionOrganizationGet, Params: map[string]string{
				"organization_id": "{{event.data.organization_id}}",
			}},
		},
	})
	assert.Contains(t, validationLocation(t, err), "does not run before")
}

func TestValidate_RejectsAnOutputTheStepDoesNotProduce(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{
		Steps: []workflow.Step{
			{ID: "org", Action: engine.ActionOrganizationGet, Params: map[string]string{
				"organization_id": "{{event.data.organization_id}}",
			}},
			{ID: "ws", Action: engine.ActionWorkspaceCreate, Params: map[string]string{
				"organization_id": "{{event.data.organization_id}}", "name": "{{steps.org.workspace_id}}",
			}},
		},
	})
	assert.Contains(t, validationLocation(t, err), "no output")
}

func TestValidate_RejectsADuplicateStepID(t *testing.T) {
	step := workflow.Step{ID: "org", Action: engine.ActionOrganizationGet, Params: map[string]string{
		"organization_id": "{{event.data.organization_id}}",
	}}
	err := newEngine(
		t,
	).Validate(string(events.OrganizationCreated), workflow.Definition{Steps: []workflow.Step{step, step}})
	assert.Contains(t, validationLocation(t, err), "used twice")
}

func TestValidate_RejectsAWorkflowConditionOnAStepOutput(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{
		Conditions: []workflow.Condition{{Field: "steps.org.name", Operator: workflow.OperatorExists}},
		Steps: []workflow.Step{{ID: "org", Action: engine.ActionOrganizationGet, Params: map[string]string{
			"organization_id": "{{event.data.organization_id}}",
		}}},
	})
	assert.Contains(t, validationLocation(t, err), "does not run before")
}

func TestValidate_RejectsAComparisonWithNoValue(t *testing.T) {
	err := newEngine(t).Validate(string(events.OrganizationCreated), workflow.Definition{
		Conditions: []workflow.Condition{{Field: "event.data.organization_id", Operator: workflow.OperatorEquals}},
		Steps: []workflow.Step{{ID: "org", Action: engine.ActionOrganizationGet, Params: map[string]string{
			"organization_id": "{{event.data.organization_id}}",
		}}},
	})
	assert.Contains(t, validationLocation(t, err), "compares against a value")
}

func TestExecute_SkipsTheRunWhenAConditionDoesNotHold(t *testing.T) {
	run := newEngine(t).Execute(context.Background(), engine.Execution{
		Workflow: workflow.Workflow{ID: "wf_1", Definition: workflow.Definition{
			Conditions: []workflow.Condition{
				{Field: "event.data.organization_id", Operator: workflow.OperatorEquals, Value: "org_2"},
			},
			Steps: []workflow.Step{{ID: "ws", Action: engine.ActionWorkspaceCreate, Params: map[string]string{
				"organization_id": "{{event.data.organization_id}}", "name": "General",
			}}},
		}},
		EventData: map[string]string{"organization_id": "org_1"},
		Trigger:   workflow.RunTriggerEvent,
	})

	assert.Equal(t, workflow.RunStatusSkipped, run.Status)
	assert.Empty(t, run.Steps)
	assert.NotNil(t, run.FinishedAt)
}

func TestExecute_DryRunResolvesWritesAndChainsPlaceholders(t *testing.T) {
	run := newEngine(t).Execute(context.Background(), engine.Execution{
		Workflow: workflow.Workflow{ID: "wf_1", Definition: workflow.Definition{Steps: []workflow.Step{
			{ID: "ws", Action: engine.ActionWorkspaceCreate, Params: map[string]string{
				"organization_id": "{{event.data.organization_id}}", "name": "General",
			}},
			{ID: "rename", Action: engine.ActionOrganizationUpdate, Params: map[string]string{
				"organization_id": "{{event.data.organization_id}}",
				"metadata":        `{"default_workspace": "{{steps.ws.workspace_id}}"}`,
			}},
			{
				ID:     "never",
				Action: engine.ActionWorkspaceCreate,
				Params: map[string]string{"organization_id": "{{event.data.organization_id}}", "name": "Other"},
				When: []workflow.Condition{
					{Field: "event.data.organization_id", Operator: workflow.OperatorEquals, Value: "nope"},
				},
			},
		}}},
		EventData: map[string]string{"organization_id": "org_1"},
		Trigger:   workflow.RunTriggerDryRun,
	})

	require.Equal(t, workflow.RunStatusSucceeded, run.Status, run.Error)
	require.Len(t, run.Steps, 3)
	assert.Equal(t, workflow.StepStatusSimulated, run.Steps[0].Status)
	assert.Equal(t, "org_1", run.Steps[0].Params["organization_id"])
	assert.JSONEq(t, `{"default_workspace": "<ws.workspace_id>"}`, run.Steps[1].Params["metadata"].(string))
	assert.Equal(t, workflow.StepStatusSkipped, run.Steps[2].Status)
}

func TestExecute_StopsAtAFailedStepUnlessItContinues(t *testing.T) {
	definition := func(continueOnError bool) workflow.Definition {
		return workflow.Definition{Steps: []workflow.Step{
			{
				ID:              "broken",
				Action:          engine.ActionWorkspaceCreate,
				ContinueOnError: continueOnError,
				Params: map[string]string{
					"organization_id": "{{event.data.organization_id}}", "name": "{{event.data.missing}}",
				},
			},
			{ID: "next", Action: engine.ActionWorkspaceCreate, Params: map[string]string{
				"organization_id": "{{event.data.organization_id}}", "name": "General",
			}},
		}}
	}
	execute := func(continueOnError bool) workflow.Run {
		return newEngine(t).Execute(context.Background(), engine.Execution{
			Workflow:  workflow.Workflow{ID: "wf_1", Definition: definition(continueOnError)},
			EventData: map[string]string{"organization_id": "org_1"},
			Trigger:   workflow.RunTriggerDryRun,
		})
	}

	stopped := execute(false)
	assert.Equal(t, workflow.RunStatusFailed, stopped.Status)
	assert.Len(t, stopped.Steps, 1)
	assert.Contains(t, stopped.Steps[0].Error, "event.data.missing")

	continued := execute(true)
	assert.Equal(t, workflow.RunStatusFailed, continued.Status)
	require.Len(t, continued.Steps, 2)
	assert.Equal(t, workflow.StepStatusSimulated, continued.Steps[1].Status)
}
