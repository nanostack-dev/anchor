package service

import (
	"anchor/internal/events"

	"go.uber.org/fx"
)

//nolint:dupl // domain event registrations share struct definitions
func OrganizationEventRegistration() events.DomainRegistration {
	return events.RegisterDomain(
		events.Definition{
			Type:        events.OrganizationCreated,
			Name:        "Organization created",
			Description: "Emitted when a new organization is created.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeOrganizations,
			Theme:       events.ThemeOrganizations,
		},
		events.Definition{
			Type:        events.OrganizationUpdated,
			Name:        "Organization updated",
			Description: "Emitted when an organization's details are updated.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeOrganizations,
			Theme:       events.ThemeOrganizations,
		},
		events.Definition{
			Type:        events.OrganizationDeleted,
			Name:        "Organization deleted",
			Description: "Emitted when an organization is deleted.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeOrganizations,
			Theme:       events.ThemeOrganizations,
		},
		events.Definition{
			Type:        events.MembershipCreated,
			Name:        "Membership created",
			Description: "Emitted when a member is added to an organization.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeOrganizations,
			Theme:       events.ThemeOrganizations,
		},
		events.Definition{
			Type:        events.MembershipUpdated,
			Name:        "Membership updated",
			Description: "Emitted when an organization member's role is updated.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeOrganizations,
			Theme:       events.ThemeOrganizations,
		},
		events.Definition{
			Type:        events.MembershipDeleted,
			Name:        "Membership deleted",
			Description: "Emitted when an organization member is removed.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeOrganizations,
			Theme:       events.ThemeOrganizations,
		},
	)
}

func WorkspaceEventRegistration() events.DomainRegistration {
	return events.RegisterDomain(
		events.Definition{
			Type:        events.WorkspaceCreated,
			Name:        "Workspace created",
			Description: "Emitted when a workspace is created.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeWorkspaces,
			Theme:       events.ThemeWorkspaces,
		},
		events.Definition{
			Type:        events.WorkspaceUpdated,
			Name:        "Workspace updated",
			Description: "Emitted when a workspace is updated.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeWorkspaces,
			Theme:       events.ThemeWorkspaces,
		},
		events.Definition{
			Type:        events.WorkspaceDeleted,
			Name:        "Workspace deleted",
			Description: "Emitted when a workspace is deleted.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeWorkspaces,
			Theme:       events.ThemeWorkspaces,
		},
	)
}

func OrganizationAPIKeyEventRegistration() events.DomainRegistration {
	return events.RegisterDomain(
		events.Definition{
			Type:        events.OrganizationAPIKeyCreated,
			Name:        "API key created",
			Description: "Emitted when an organization API key is created.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeAPIKeys,
			Theme:       events.ThemeAPIKeys,
		},
		events.Definition{
			Type:        events.OrganizationAPIKeyUpdated,
			Name:        "API key updated",
			Description: "Emitted when an organization API key is updated.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeAPIKeys,
			Theme:       events.ThemeAPIKeys,
		},
		events.Definition{
			Type:        events.OrganizationAPIKeyDeleted,
			Name:        "API key deleted",
			Description: "Emitted when an organization API key is deleted.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeAPIKeys,
			Theme:       events.ThemeAPIKeys,
		},
	)
}

func ProductUserEventRegistration() events.DomainRegistration {
	return events.RegisterDomain(
		events.Definition{
			Type:        events.ProductUserCreated,
			Name:        "Product user created",
			Description: "Emitted when a product user is created.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeUsers,
			Theme:       events.ThemeUsers,
		},
		events.Definition{
			Type:        events.ProductUserUpdated,
			Name:        "Product user updated",
			Description: "Emitted when a product user is updated.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeUsers,
			Theme:       events.ThemeUsers,
		},
		events.Definition{
			Type:        events.ProductUserDeleted,
			Name:        "Product user deleted",
			Description: "Emitted when a product user is deleted.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeUsers,
			Theme:       events.ThemeUsers,
		},
	)
}

//nolint:dupl // domain event registrations share struct definitions
func ProductRBACEventRegistration() events.DomainRegistration {
	return events.RegisterDomain(
		events.Definition{
			Type:        events.ProductRoleCreated,
			Name:        "Role created",
			Description: "Emitted when a product role is created.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeRolesPermissions,
			Theme:       events.ThemeRolesPermissions,
		},
		events.Definition{
			Type:        events.ProductRoleUpdated,
			Name:        "Role updated",
			Description: "Emitted when a product role is updated or its permissions change.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeRolesPermissions,
			Theme:       events.ThemeRolesPermissions,
		},
		events.Definition{
			Type:        events.ProductRoleDeleted,
			Name:        "Role deleted",
			Description: "Emitted when a product role is deleted.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeRolesPermissions,
			Theme:       events.ThemeRolesPermissions,
		},
		events.Definition{
			Type:        events.ProductResourcePermissionCreated,
			Name:        "Resource permission created",
			Description: "Emitted when a resource permission is created.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeRolesPermissions,
			Theme:       events.ThemeRolesPermissions,
		},
		events.Definition{
			Type:        events.ProductResourcePermissionUpdated,
			Name:        "Resource permission updated",
			Description: "Emitted when a resource permission is updated.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeRolesPermissions,
			Theme:       events.ThemeRolesPermissions,
		},
		events.Definition{
			Type:        events.ProductResourcePermissionDeleted,
			Name:        "Resource permission deleted",
			Description: "Emitted when a resource permission is deleted.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   events.ThemeRolesPermissions,
			Theme:       events.ThemeRolesPermissions,
		},
	)
}

func AsDomainEventRegistration(fn any) any {
	return fx.Annotate(
		fn,
		fx.ResultTags(`group:"domain_events"`),
	)
}
