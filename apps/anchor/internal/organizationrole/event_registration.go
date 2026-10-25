package organizationrole

import "anchor/internal/events"

func EventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupRolesPermissions,
		events.Definition{
			Type:        events.OrganizationRoleUpdated,
			Name:        "Organization role updated",
			Description: "Emitted when an organization's role changes, such as becoming or ceasing to be its default role.",
		},
	)
}
