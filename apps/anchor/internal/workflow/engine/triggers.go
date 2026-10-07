package engine

import (
	"slices"
	"strings"

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
