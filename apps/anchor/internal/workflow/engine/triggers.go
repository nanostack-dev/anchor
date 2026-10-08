package engine

import (
	"encoding/json"
	"slices"
	"strings"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
)

// Trigger is a catalog event a workflow can start on, with the typed keys
// its thin payload carries under `event.data`. DataFields repeats their names.
type Trigger struct {
	events.Definition
	Fields     []FieldSpec
	DataFields []string
}

func fieldNames(fields []FieldSpec) []string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.Name)
	}
	return names
}

func dataFieldsOf(eventType events.Type) []string {
	name := string(eventType)
	switch {
	case strings.HasPrefix(name, "organization.membership."):
		return []string{events.FieldOrganizationID, events.FieldProductUserID}
	case strings.HasPrefix(name, "organization.invitation."):
		return []string{events.FieldOrganizationID, events.FieldInvitationID}
	case strings.HasPrefix(name, "organization.api_key."):
		return []string{events.FieldOrganizationID, events.FieldAPIKeyID}
	case strings.HasPrefix(name, "workspace."):
		return []string{events.FieldOrganizationID, events.FieldWorkspaceID}
	case strings.HasPrefix(name, "organization."):
		return []string{events.FieldOrganizationID}
	case strings.HasPrefix(name, "product_user."), strings.HasPrefix(name, "clerk.user."):
		return []string{events.FieldProductUserID}
	case strings.HasPrefix(name, "product.role."):
		return []string{events.FieldRoleID}
	case strings.HasPrefix(name, "product.resource_permission."):
		return []string{events.FieldPermissionName}
	}
	return []string{}
}

func (e *Engine) customTriggers(workflows []workflow.Workflow) []Trigger {
	fields := map[string][]FieldSpec{}
	var order []string
	note := func(eventType string) {
		if _, seen := fields[eventType]; !seen {
			order = append(order, eventType)
			fields[eventType] = []FieldSpec{}
		}
	}
	for _, wf := range workflows {
		for _, step := range wf.Definition.Steps {
			if step.Action != ActionWorkflowEmit {
				continue
			}
			eventType := workflow.CustomEventType(step.Params[keyEvent])
			note(eventType)
			for _, field := range emittedFields(step.Params) {
				if !slices.ContainsFunc(
					fields[eventType],
					func(known FieldSpec) bool { return known.Name == field.Name },
				) {
					field.Description = "Sent by the steps that emit this event."
					fields[eventType] = append(fields[eventType], field)
				}
			}
		}
		if workflow.IsCustomEvent(wf.TriggerEventType) {
			note(wf.TriggerEventType)
		}
	}
	slices.Sort(order)
	triggers := make([]Trigger, 0, len(order))
	for _, eventType := range order {
		slices.SortFunc(fields[eventType], func(a, b FieldSpec) int { return strings.Compare(a.Name, b.Name) })
		triggers = append(triggers, Trigger{
			Type:        events.Type(eventType),
			Name:        strings.TrimPrefix(eventType, workflow.CustomEventPrefix),
			Description: "Custom event emitted by a workflow step.",
			GroupType:   events.GroupTypeCustom,
			GroupName:   GroupCustom,
			Fields:      fields[eventType],
			DataFields:  fieldNames(fields[eventType]),
		})
	}
	return triggers
}

// declaredKeys reads the top-level keys of a JSON object parameter whose
// values may still hold {{ }} references.
func declaredKeys(raw string) []string {
	neutral := referencePattern.ReplaceAllString(raw, "null")
	var object map[string]any
	if json.Unmarshal([]byte(neutral), &object) != nil {
		return nil
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func triggersOf(catalog events.Catalog) []Trigger {
	definitions := catalog.All()
	triggers := make([]Trigger, 0, len(definitions))
	for _, definition := range definitions {
		fields := fieldsOf(definition.Type)
		triggers = append(triggers, Trigger{Definition: definition, Fields: fields, DataFields: fieldNames(fields)})
	}
	slices.SortStableFunc(triggers, func(a, b Trigger) int {
		return strings.Compare(string(a.Type), string(b.Type))
	})
	return triggers
}
