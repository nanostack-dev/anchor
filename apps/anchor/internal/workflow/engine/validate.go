package engine

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
)

var stepIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// Validate checks a definition against the action catalog and the trigger's
// payload: every action and operator is known, every required parameter is
// set, and every reference names a value that exists by the time it is read.
func (e *Engine) Validate(triggerType string, definition workflow.Definition) error {
	custom := workflow.IsCustomEvent(triggerType)
	if custom && !workflow.ValidCustomEvent(triggerType) {
		return InvalidDefinitionError("trigger_event_type", fmt.Sprintf(
			"%q is not a valid custom event: use custom. followed by lowercase words joined by dots, "+
				"such as custom.onboarding.completed.", triggerType))
	}
	if !custom && !e.catalog.IsKnown(events.Type(triggerType)) {
		return InvalidDefinitionError("trigger_event_type",
			fmt.Sprintf("%q is not an event in the product event catalog.", triggerType))
	}
	if len(definition.Steps) == 0 {
		return InvalidDefinitionError("steps", "A workflow needs at least one step.")
	}
	if len(definition.Steps) > workflow.MaxSteps {
		return InvalidDefinitionError("steps", fmt.Sprintf("A workflow has at most %d steps.", workflow.MaxSteps))
	}
	known := referenceRoots{
		dataFields:   dataFieldsOf(events.Type(triggerType)),
		anyDataField: custom,
		steps:        map[string]ActionSpec{},
	}
	if err := validateConditions("conditions", definition.Conditions, known); err != nil {
		return err
	}
	for index, step := range definition.Steps {
		location := fmt.Sprintf("steps[%d]", index)
		if err := e.validateStep(location, step, known); err != nil {
			return err
		}
		known.steps[step.ID] = e.actions[step.Action].spec
	}
	return nil
}

func (e *Engine) validateStep(location string, step workflow.Step, known referenceRoots) error {
	if !stepIDPattern.MatchString(step.ID) {
		return InvalidDefinitionError(location+".id",
			"A step id starts with a lowercase letter and holds only lowercase letters, digits and underscores.")
	}
	if _, taken := known.steps[step.ID]; taken {
		return InvalidDefinitionError(location+".id", fmt.Sprintf("Step id %q is used twice.", step.ID))
	}
	found, ok := e.actions[step.Action]
	if !ok {
		return InvalidDefinitionError(location+".action", fmt.Sprintf("%q is not a workflow action.", step.Action))
	}
	for name := range step.Params {
		if _, declared := found.spec.Param(name); !declared {
			return InvalidDefinitionError(location+".params."+name,
				fmt.Sprintf("Action %s has no parameter %q.", step.Action, name))
		}
	}
	for _, param := range found.spec.Params {
		raw := step.Params[param.Name]
		if param.Required && strings.TrimSpace(raw) == "" {
			return InvalidDefinitionError(location+".params."+param.Name,
				fmt.Sprintf("Parameter %q of action %s is required.", param.Name, step.Action))
		}
		if err := validateParamValue(location+".params."+param.Name, param, raw); err != nil {
			return err
		}
		if err := known.check(location+".params."+param.Name, raw); err != nil {
			return err
		}
	}
	return validateConditions(location+".when", step.When, known)
}

func validateConditions(location string, conditions []workflow.Condition, known referenceRoots) error {
	if len(conditions) > workflow.MaxConditions {
		return InvalidDefinitionError(location,
			fmt.Sprintf("A condition list holds at most %d conditions.", workflow.MaxConditions))
	}
	for index, condition := range conditions {
		conditionLocation := fmt.Sprintf("%s[%d]", location, index)
		if !slices.Contains(workflow.AllOperators(), condition.Operator) {
			return InvalidDefinitionError(conditionLocation+".operator",
				fmt.Sprintf("%q is not a condition operator.", condition.Operator))
		}
		if err := known.checkPath(conditionLocation+".field", strings.TrimSpace(condition.Field)); err != nil {
			return err
		}
		if condition.Operator.NeedsValue() && condition.Value == "" {
			return InvalidDefinitionError(conditionLocation+".value",
				fmt.Sprintf("Operator %s compares against a value.", condition.Operator))
		}
		if err := known.check(conditionLocation+".value", condition.Value); err != nil {
			return err
		}
	}
	return nil
}

func validateParamValue(location string, param ParamSpec, raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if param.Literal && len(References(value)) > 0 {
		return InvalidDefinitionError(location,
			fmt.Sprintf("Parameter %q must be written out: it cannot hold {{ }} references.", param.Name))
	}
	if param.Type == ParamCustomEvent && !workflow.ValidCustomEvent(workflow.CustomEventType(value)) {
		return InvalidDefinitionError(location, fmt.Sprintf(
			"%q is not a valid custom event name: use lowercase words joined by dots, such as onboarding.completed.",
			value))
	}
	if len(param.Options) > 0 && len(References(value)) == 0 &&
		!slices.ContainsFunc(param.Options, func(option string) bool { return strings.EqualFold(option, value) }) {
		return InvalidDefinitionError(location, fmt.Sprintf(
			"Parameter %q accepts %s.", param.Name, strings.Join(param.Options, ", ")))
	}
	return nil
}

type referenceRoots struct {
	dataFields   []string
	anyDataField bool
	steps        map[string]ActionSpec
}

func (r referenceRoots) check(location, raw string) error {
	for _, path := range References(raw) {
		if err := r.checkPath(location, path); err != nil {
			return err
		}
	}
	return nil
}

func (r referenceRoots) checkPath(location, path string) error {
	segments := strings.Split(path, ".")
	switch {
	case path == "event.id", path == "event.type", path == "workflow.id", path == "workflow.name":
		return nil
	case len(segments) == 3 && segments[0] == keyEvent && segments[1] == "data":
		if r.anyDataField || slices.Contains(r.dataFields, segments[2]) {
			return nil
		}
		return InvalidDefinitionError(location, fmt.Sprintf(
			"The trigger event carries no %q. It carries: %s.", segments[2], strings.Join(r.dataFields, ", ")))
	case len(segments) >= 3 && segments[0] == "steps":
		spec, ran := r.steps[segments[1]]
		if !ran {
			return InvalidDefinitionError(location, fmt.Sprintf(
				"%q reads step %q, which does not run before this point.", path, segments[1]))
		}
		if slices.ContainsFunc(spec.Outputs, func(output OutputSpec) bool { return output.Name == segments[2] }) {
			return nil
		}
		return InvalidDefinitionError(location, fmt.Sprintf(
			"Step %q (%s) has no output %q.", segments[1], spec.Type, segments[2]))
	}
	return InvalidDefinitionError(location, fmt.Sprintf(
		"%q is not a value a workflow can read. Use event.data.<field>, steps.<id>.<output>, event.type or workflow.id.",
		path,
	))
}
