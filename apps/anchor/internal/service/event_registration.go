package service

import "anchor/internal/events"

func OrganizationEventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupOrganizations,
		events.Definition{
			Type:        events.OrganizationCreated,
			Name:        "Organization created",
			Description: "Emitted when a new organization is created.",
		},
		events.Definition{
			Type:        events.OrganizationUpdated,
			Name:        "Organization updated",
			Description: "Emitted when an organization's details are updated.",
		},
		events.Definition{
			Type:        events.OrganizationDeleted,
			Name:        "Organization deleted",
			Description: "Emitted when an organization is deleted.",
		},
		events.Definition{
			Type:        events.MembershipCreated,
			Name:        "Membership created",
			Description: "Emitted when a member is added to an organization.",
		},
		events.Definition{
			Type:        events.MembershipUpdated,
			Name:        "Membership updated",
			Description: "Emitted when an organization member's role is updated.",
		},
		events.Definition{
			Type:        events.MembershipDeleted,
			Name:        "Membership deleted",
			Description: "Emitted when an organization member is removed.",
		},
	)
}

func WorkspaceEventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupWorkspaces,
		events.Definition{
			Type:        events.WorkspaceCreated,
			Name:        "Workspace created",
			Description: "Emitted when a workspace is created.",
		},
		events.Definition{
			Type:        events.WorkspaceUpdated,
			Name:        "Workspace updated",
			Description: "Emitted when a workspace is updated.",
		},
		events.Definition{
			Type:        events.WorkspaceDeleted,
			Name:        "Workspace deleted",
			Description: "Emitted when a workspace is deleted.",
		},
	)
}

func OrganizationAPIKeyEventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupAPIKeys,
		events.Definition{
			Type:        events.OrganizationAPIKeyCreated,
			Name:        "API key created",
			Description: "Emitted when an organization API key is created.",
		},
		events.Definition{
			Type:        events.OrganizationAPIKeyUpdated,
			Name:        "API key updated",
			Description: "Emitted when an organization API key is updated.",
		},
		events.Definition{
			Type:        events.OrganizationAPIKeyDeleted,
			Name:        "API key deleted",
			Description: "Emitted when an organization API key is deleted.",
		},
	)
}

func ProductUserEventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupUsers,
		events.Definition{
			Type:        events.ProductUserCreated,
			Name:        "Product user created",
			Description: "Emitted when a product user is created.",
		},
		events.Definition{
			Type:        events.ProductUserUpdated,
			Name:        "Product user updated",
			Description: "Emitted when a product user is updated.",
		},
		events.Definition{
			Type:        events.ProductUserDeleted,
			Name:        "Product user deleted",
			Description: "Emitted when a product user is deleted.",
		},
	)
}

func ProductRBACEventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupRolesPermissions,
		events.Definition{
			Type:        events.ProductRoleCreated,
			Name:        "Role created",
			Description: "Emitted when a product role is created.",
		},
		events.Definition{
			Type:        events.ProductRoleUpdated,
			Name:        "Role updated",
			Description: "Emitted when a product role is updated or its permissions change.",
		},
		events.Definition{
			Type:        events.ProductRoleDeleted,
			Name:        "Role deleted",
			Description: "Emitted when a product role is deleted.",
		},
		events.Definition{
			Type:        events.ProductResourcePermissionCreated,
			Name:        "Resource permission created",
			Description: "Emitted when a resource permission is created.",
		},
		events.Definition{
			Type:        events.ProductResourcePermissionUpdated,
			Name:        "Resource permission updated",
			Description: "Emitted when a resource permission is updated.",
		},
		events.Definition{
			Type:        events.ProductResourcePermissionDeleted,
			Name:        "Resource permission deleted",
			Description: "Emitted when a resource permission is deleted.",
		},
	)
}
