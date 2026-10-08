package engine

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
)

// FieldType says what a value carried by an event or produced by a step is,
// so a client offers a parameter only the fields that fit it. Every value is
// still sent as a string; the type is a promise about its content.
type FieldType string

const (
	FieldText            FieldType = "text"
	FieldEmail           FieldType = "email"
	FieldNumber          FieldType = "number"
	FieldBoolean         FieldType = "boolean"
	FieldTimestamp       FieldType = "timestamp"
	FieldURL             FieldType = "url"
	FieldJSON            FieldType = "json"
	FieldOrganization    FieldType = "organization"
	FieldWorkspace       FieldType = "workspace"
	FieldProductUser     FieldType = "product_user"
	FieldInvitation      FieldType = "invitation"
	FieldAPIKey          FieldType = "api_key"
	FieldRole            FieldType = "role"
	FieldPermission      FieldType = "permission"
	FieldLicense         FieldType = "license"
	FieldLicenseTemplate FieldType = "license_template"
)

func AllFieldTypes() []FieldType {
	return []FieldType{
		FieldText, FieldEmail, FieldNumber, FieldBoolean, FieldTimestamp, FieldURL, FieldJSON,
		FieldOrganization, FieldWorkspace, FieldProductUser, FieldInvitation, FieldAPIKey,
		FieldRole, FieldPermission, FieldLicense, FieldLicenseTemplate,
	}
}

func (t FieldType) Valid() bool {
	return slices.Contains(AllFieldTypes(), t)
}

// FieldSpec is one value an event carries under event.data, or one output a
// step produces.
type FieldSpec struct {
	Name        string
	Type        FieldType
	Description string
}

func catalogEventField(name string) FieldSpec {
	types := map[string]FieldType{
		events.FieldOrganizationID: FieldOrganization,
		events.FieldProductUserID:  FieldProductUser,
		events.FieldWorkspaceID:    FieldWorkspace,
		events.FieldInvitationID:   FieldInvitation,
		events.FieldAPIKeyID:       FieldAPIKey,
		events.FieldRoleID:         FieldRole,
		events.FieldPermissionName: FieldPermission,
	}
	descriptions := map[string]string{
		events.FieldOrganizationID: "Identifier of the organization.",
		events.FieldProductUserID:  "Identifier of the product user.",
		events.FieldWorkspaceID:    "Identifier of the workspace.",
		events.FieldInvitationID:   "Identifier of the invitation.",
		events.FieldAPIKeyID:       "Identifier of the organization API key.",
		events.FieldRoleID:         "Identifier of the product role.",
		events.FieldPermissionName: "Name of the resource permission.",
	}
	return FieldSpec{Name: name, Type: types[name], Description: descriptions[name]}
}

func fieldsOf(eventType events.Type) []FieldSpec {
	names := dataFieldsOf(eventType)
	fields := make([]FieldSpec, 0, len(names))
	for _, name := range names {
		fields = append(fields, catalogEventField(name))
	}
	return fields
}

// declaredTypes reads a step's data_types parameter: the type of each key of
// its data parameter. A key it leaves out is text.
func declaredTypes(raw string) map[string]FieldType {
	types := map[string]FieldType{}
	if raw == "" {
		return types
	}
	var declared map[string]string
	if json.Unmarshal([]byte(raw), &declared) != nil {
		return types
	}
	for key, value := range declared {
		types[key] = FieldType(value)
	}
	return types
}

// emittedFields lists the typed fields one workflow.emit step sends.
func emittedFields(params map[string]string) []FieldSpec {
	types := declaredTypes(params[keyDataTypes])
	keys := declaredKeys(params[keyData])
	fields := make([]FieldSpec, 0, len(keys))
	for _, key := range keys {
		fieldType, typed := types[key]
		if !typed {
			fieldType = FieldText
		}
		fields = append(fields, FieldSpec{Name: key, Type: fieldType})
	}
	return fields
}

// FieldConflict is a custom event field two steps send with different types.
type FieldConflict struct {
	Event        string
	Field        string
	Type         FieldType
	OtherType    FieldType
	StepIndex    int
	OtherID      string
	OtherName    string
	OtherStepID  string
	OtherIsOwner bool
}

// FieldTypeConflict finds the first custom event field the candidate sends
// with a type another workflow of the product, or another of its own steps,
// sends differently. Every workflow counts, enabled or not, because the
// catalog types a custom event from all of them.
func (e *Engine) FieldTypeConflict(candidate workflow.Workflow, others []workflow.Workflow) *FieldConflict {
	type sender struct {
		fieldType FieldType
		workflow  workflow.Workflow
		stepID    string
	}
	seen := map[string]map[string]sender{}
	record := func(wf workflow.Workflow, step workflow.Step) {
		eventType := workflow.CustomEventType(step.Params[keyEvent])
		if seen[eventType] == nil {
			seen[eventType] = map[string]sender{}
		}
		for _, field := range emittedFields(step.Params) {
			if _, known := seen[eventType][field.Name]; !known {
				seen[eventType][field.Name] = sender{fieldType: field.Type, workflow: wf, stepID: step.ID}
			}
		}
	}
	for _, other := range others {
		if other.ID == candidate.ID {
			continue
		}
		for _, step := range other.Definition.Steps {
			if step.Action == ActionWorkflowEmit {
				record(other, step)
			}
		}
	}
	for index, step := range candidate.Definition.Steps {
		if step.Action != ActionWorkflowEmit {
			continue
		}
		eventType := workflow.CustomEventType(step.Params[keyEvent])
		for _, field := range emittedFields(step.Params) {
			first, known := seen[eventType][field.Name]
			if known && first.fieldType != field.Type {
				return &FieldConflict{
					Event: eventType, Field: field.Name, Type: field.Type, OtherType: first.fieldType,
					StepIndex: index, OtherID: first.workflow.ID, OtherName: first.workflow.Name,
					OtherStepID: first.stepID, OtherIsOwner: first.workflow.ID == candidate.ID,
				}
			}
		}
		record(candidate, step)
	}
	return nil
}

func (c FieldConflict) Describe() string {
	where := fmt.Sprintf("“%s”", c.OtherName)
	if c.OtherIsOwner {
		where = fmt.Sprintf("step %q of this workflow", c.OtherStepID)
	}
	return fmt.Sprintf("%s carries %q as %s here, but %s sends it as %s. "+
		"Every step that sends an event gives a field the same type.", c.Event, c.Field, c.Type, where, c.OtherType)
}

func validateFieldTypes(location, raw string, describes map[string]bool) error {
	var declared map[string]string
	if err := json.Unmarshal([]byte(raw), &declared); err != nil {
		return InvalidDefinitionError(location,
			`Field types are a JSON object of field to type, such as {"plan": "text"}.`)
	}
	for _, key := range slices.Sorted(maps.Keys(declared)) {
		if !FieldType(declared[key]).Valid() {
			return InvalidDefinitionError(location, fmt.Sprintf("%q is not a field type for %q.", declared[key], key))
		}
		if !describes[key] {
			return InvalidDefinitionError(location, fmt.Sprintf("%q is typed but the event data has no such key.", key))
		}
	}
	return nil
}
