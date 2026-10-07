package engine

import (
	"encoding/json"
	"slices"
	"strings"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
)

// Trigger is a catalog event a workflow can start on, with the keys its
// thin payload carries under `event.data`.
type Trigger struct {
	events.Definition
	DataFields []string
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
	fields := map[string][]string{}
	var order []string
	for _, wf := range workflows {
		for _, step := range wf.Definition.Steps {
			if step.Action != ActionWorkflowEmit {
				continue
			}
			eventType := workflow.CustomEventType(step.Params[keyEvent])
			if _, seen := fields[eventType]; !seen {
				order = append(order, eventType)
				fields[eventType] = []string{}
			}
			for _, key := range declaredKeys(step.Params[keyData]) {
				if !slices.Contains(fields[eventType], key) {
					fields[eventType] = append(fields[eventType], key)
				}
			}
		}
		if workflow.IsCustomEvent(wf.TriggerEventType) {
			if _, seen := fields[wf.TriggerEventType]; !seen {
				order = append(order, wf.TriggerEventType)
				fields[wf.TriggerEventType] = []string{}
			}
		}
	}
	slices.Sort(order)
	triggers := make([]Trigger, 0, len(order))
	for _, eventType := range order {
		slices.Sort(fields[eventType])
		triggers = append(triggers, Trigger{
			Type:        events.Type(eventType),
			Name:        strings.TrimPrefix(eventType, workflow.CustomEventPrefix),
			Description: "Custom event emitted by a workflow step.",
			GroupType:   events.GroupTypeCustom,
			GroupName:   GroupCustom,
			DataFields:  fields[eventType],
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
		triggers = append(triggers, Trigger{Definition: definition, DataFields: dataFieldsOf(definition.Type)})
	}
	slices.SortStableFunc(triggers, func(a, b Trigger) int {
		return strings.Compare(string(a.Type), string(b.Type))
	})
	return triggers
}
