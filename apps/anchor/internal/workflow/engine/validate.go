package engine

import (
	"fmt"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

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
	problems := &problemList{}
	problems.add(validateConditions("conditions", definition.Conditions, known))
	for index, step := range definition.Steps {
		location := fmt.Sprintf("steps[%d]", index)
		if err := e.validateStep(location, step, known, problems); err != nil {
			return err
		}
		known.steps[step.ID] = e.actions[step.Action].spec
	}
	return problems.err()
}

// problemList gathers the problems of every step's parameters and
// conditions, so one save reports all of them.
type problemList struct {
	details []fault.Detail
}

func (p *problemList) add(err error) {
	if err == nil {
		return
	}
	if found, ok := fault.As(err); ok {
		p.details = append(p.details, found.Details...)
		return
	}
	p.details = append(p.details, fault.Detail{Code: invalidDefinitionCode, Message: err.Error()})
}

func (p *problemList) err() error {
	if len(p.details) == 0 {
		return nil
	}
	return fault.NewWithDetails(p.details, http.StatusBadRequest)
}

// validateStep fails fast on what makes the step unreadable (its id or its
// action), and adds every parameter and condition problem to problems.
func (e *Engine) validateStep(
	location string, step workflow.Step, known referenceRoots, problems *problemList,
) error {
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
	for _, name := range slices.Sorted(maps.Keys(step.Params)) {
		if _, declared := found.spec.Param(name); !declared {
			problems.add(InvalidDefinitionError(location+".params."+name,
				fmt.Sprintf("“%s” has no parameter %q.", found.spec.Name, name)))
		}
	}
	for _, param := range found.spec.Params {
		paramLocation := location + ".params." + param.Name
		raw := step.Params[param.Name]
		if param.Required && strings.TrimSpace(raw) == "" {
			problems.add(InvalidDefinitionError(paramLocation,
				fmt.Sprintf("%s is required for “%s”.", param.Label, found.spec.Name)))
			continue
		}
		if err := validateParamValue(paramLocation, param, raw); err != nil {
			problems.add(err)
			continue
		}
		problems.add(known.check(paramLocation, raw))
	}
	problems.add(validateConditions(location+".when", step.When, known))
	return nil
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
			fmt.Sprintf("%s must be written out: it cannot hold {{ }} references.", param.Label))
	}
	if param.Type == ParamCustomEvent && !workflow.ValidCustomEvent(workflow.CustomEventType(value)) {
		return InvalidDefinitionError(location, fmt.Sprintf(
			"%q is not a valid custom event name: use lowercase words joined by dots, such as onboarding.completed.",
			value))
	}
	if len(param.Options) > 0 && len(References(value)) == 0 &&
		!slices.ContainsFunc(param.Options, func(option string) bool { return strings.EqualFold(option, value) }) {
		return InvalidDefinitionError(location, fmt.Sprintf(
			"%s accepts %s.", param.Label, strings.Join(param.Options, ", ")))
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
