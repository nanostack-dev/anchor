package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
)

type Engine struct {
	actions map[workflow.ActionType]action
	specs   []ActionSpec
	catalog events.Catalog
	now     func() time.Time
}

func New(services Services, catalog events.Catalog) *Engine {
	built := buildActions(services)
	actions := make(map[workflow.ActionType]action, len(built))
	specs := make([]ActionSpec, 0, len(built))
	for _, candidate := range built {
		actions[candidate.spec.Type] = candidate
		specs = append(specs, candidate.spec)
	}
	return &Engine{actions: actions, specs: specs, catalog: catalog, now: time.Now}
}

func (e *Engine) Actions() []ActionSpec {
	return e.specs
}

func (e *Engine) Triggers() []Trigger {
	return triggersOf(e.catalog)
}

// Execution is one event delivered to one workflow.
type Execution struct {
	Workflow  workflow.Workflow
	RunID     string
	EventID   string
	EventType string
	EventData map[string]string
	Trigger   workflow.RunTrigger
	// Depth is the causation depth of the event. Every write the run makes
	// emits its events one level deeper.
	Depth int
}

// Execute runs the workflow's steps in order and reports what each did. A
// dry run reads for real and only resolves the steps that write.
func (e *Engine) Execute(ctx context.Context, execution Execution) workflow.Run {
	run := e.execute(ctx, execution)
	finished := e.now()
	run.FinishedAt = &finished
	return run
}

func (e *Engine) execute(ctx context.Context, execution Execution) workflow.Run {
	dryRun := execution.Trigger == workflow.RunTriggerDryRun
	run := workflow.Run{
		ID:         execution.RunID,
		WorkflowID: execution.Workflow.ID,
		ProductID:  execution.Workflow.ProductID,
		EventID:    execution.EventID,
		EventType:  execution.EventType,
		EventData:  execution.EventData,
		Trigger:    execution.Trigger,
		Status:     workflow.RunStatusSucceeded,
		Steps:      []workflow.StepResult{},
		StartedAt:  e.now(),
	}
	ctx = events.WithCausationDepth(ctx, execution.Depth+1)
	scope := NewScope(
		execution.Workflow.ID, execution.Workflow.Name,
		execution.EventID, execution.EventType, execution.EventData,
	)
	env := Env{TenantID: execution.Workflow.PlatformTenantID, ProductID: execution.Workflow.ProductID, RunID: run.ID}

	matched, err := scope.Holds(execution.Workflow.Definition.Conditions)
	if err != nil {
		return failRun(run, "conditions: "+err.Error())
	}
	if !matched {
		run.Status = workflow.RunStatusSkipped
		return run
	}

	var failures []string
	for _, step := range execution.Workflow.Definition.Steps {
		result := e.executeStep(ctx, env, scope, step, dryRun)
		run.Steps = append(run.Steps, result)
		if result.Status != workflow.StepStatusFailed {
			continue
		}
		failures = append(failures, fmt.Sprintf("%s: %s", step.ID, result.Error))
		if !step.ContinueOnError {
			break
		}
	}
	if len(failures) > 0 {
		return failRun(run, strings.Join(failures, "; "))
	}
	return run
}

func (e *Engine) executeStep(
	ctx context.Context, env Env, scope Scope, step workflow.Step, dryRun bool,
) workflow.StepResult {
	result := workflow.StepResult{StepID: step.ID, Action: step.Action}
	selected, ok := e.actions[step.Action]
	if !ok {
		return failStep(result, fmt.Errorf("%q is not a workflow action", step.Action))
	}
	applies, err := scope.Holds(step.When)
	if err != nil {
		return failStep(result, err)
	}
	if !applies {
		result.Status = workflow.StepStatusSkipped
		return result
	}

	params := Params{}
	for _, spec := range selected.spec.Params {
		raw, given := step.Params[spec.Name]
		if !given {
			continue
		}
		value, renderErr := scope.Render(raw)
		if renderErr != nil {
			return failStep(result, fmt.Errorf("parameter %q: %w", spec.Name, renderErr))
		}
		params[spec.Name] = value
		if spec.Required && params.String(spec.Name) == "" {
			return failStep(result, missingParamError(spec.Name))
		}
	}
	result.Params = params

	if dryRun && selected.spec.Writes {
		result.Status = workflow.StepStatusSimulated
		result.Output = simulatedOutput(step.ID, selected.spec)
		scope.SetStepOutput(step.ID, result.Output)
		return result
	}

	stepEnv := env
	stepEnv.StepID = step.ID
	output, err := safeRun(ctx, selected.run, stepEnv, params)
	if err != nil {
		return failStep(result, err)
	}
	result.Status = workflow.StepStatusSucceeded
	result.Output = output
	scope.SetStepOutput(step.ID, output)
	return result
}

func safeRun(ctx context.Context, run runFunc, env Env, params Params) (map[string]any, error) {
	var output map[string]any
	var err error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("action panicked: %v", recovered)
			}
		}()
		output, err = run(ctx, env, params)
	}()
	return output, err
}

func simulatedOutput(stepID string, spec ActionSpec) map[string]any {
	output := make(map[string]any, len(spec.Outputs))
	for _, field := range spec.Outputs {
		output[field.Name] = fmt.Sprintf("<%s.%s>", stepID, field.Name)
	}
	return output
}

func failStep(result workflow.StepResult, err error) workflow.StepResult {
	result.Status = workflow.StepStatusFailed
	result.Error = err.Error()
	return result
}

func failRun(run workflow.Run, message string) workflow.Run {
	run.Status = workflow.RunStatusFailed
	run.Error = &message
	return run
}
