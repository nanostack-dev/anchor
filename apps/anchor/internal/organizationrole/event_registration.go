package organizationrole

import "anchor/internal/events"

func EventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupRolesPermissions,
		events.Definition{
			Type:        events.OrganizationRoleCreated,
			Name:        "Organization role created",
			Description: "Emitted when an organization gets a custom role, created by it or left by a deleted role template.",
		},
		events.Definition{
			Type:        events.OrganizationRoleUpdated,
			Name:        "Organization role updated",
			Description: "Emitted when an organization's role changes, such as becoming or ceasing to be its default role.",
		},
		events.Definition{
			Type:        events.OrganizationRoleDeleted,
			Name:        "Organization role deleted",
			Description: "Emitted when an organization deletes one of its custom roles.",
		},
		events.Definition{
			Type:        events.OrganizationRoleCopied,
			Name:        "Organization role copied",
			Description: "Emitted inside Anchor when a write copies a role template into an organization.",
			Undelivered: true,
		},
	)
}
