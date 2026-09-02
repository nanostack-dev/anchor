package events

import (
	"slices"

	"anchor/internal/integration/provider"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"go.uber.org/fx"
)

type GroupType string

const (
	GroupTypeTheme       GroupType = "theme"
	GroupTypeIntegration GroupType = "integration"
)

const (
	ThemeOrganizations    = "Organizations"
	ThemeWorkspaces       = "Workspaces"
	ThemeAPIKeys          = "API Keys"
	ThemeUsers            = "Users"
	ThemeLicensing        = "Licensing"
	ThemeRolesPermissions = "Roles & Permissions"
	IntegrationClerk      = "CLERK"
)

type Definition struct {
	Type        Type      `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	GroupType   GroupType `json:"group_type"`
	GroupName   string    `json:"group_name"`
	Theme       string    `json:"theme,omitempty"`
	Integration string    `json:"integration,omitempty"`
}

type DomainRegistration struct {
	Definitions []Definition
}

func RegisterDomain(definitions ...Definition) DomainRegistration {
	return DomainRegistration{Definitions: definitions}
}

type Catalog interface {
	All() []Definition
	Types() []Type
	IsKnown(t Type) bool
	AllEventTypesStrings() []string
}

type CatalogParams struct {
	fx.In
	DomainRegistrations []DomainRegistration `group:"domain_events"`
	Providers           []provider.Provider  `group:"integration_providers"`
}

type catalog struct {
	definitions []Definition
	byType      map[Type]Definition
}

func NewCatalog(p CatalogParams) Catalog {
	var defs []Definition
	for _, reg := range p.DomainRegistrations {
		defs = append(defs, reg.Definitions...)
	}
	for _, prov := range p.Providers {
		if _, ok := prov.(provider.WebhookIngestor); !ok {
			continue
		}
		if eventProv, ok := prov.(provider.WebhookEventProvider); ok {
			for _, ev := range eventProv.WebhookEvents() {
				defs = append(defs, Definition{
					Type:        Type(ev.Type),
					Name:        ev.Name,
					Description: ev.Description,
					GroupType:   GroupTypeIntegration,
					GroupName:   prov.Type(),
					Integration: prov.Type(),
				})
			}
		}
	}
	if len(defs) == 0 {
		defs = DefaultDefinitions()
	}
	byType := make(map[Type]Definition, len(defs))
	for _, d := range defs {
		byType[d.Type] = d
	}
	return &catalog{
		definitions: defs,
		byType:      byType,
	}
}

func (c *catalog) All() []Definition {
	return slices.Clone(c.definitions)
}

func (c *catalog) Types() []Type {
	return functional.Slice(c.definitions).Map(func(d Definition) Type {
		return d.Type
	})
}

func (c *catalog) IsKnown(t Type) bool {
	_, ok := c.byType[t]
	return ok
}

func (c *catalog) AllEventTypesStrings() []string {
	return functional.Slice(c.definitions).Map(func(d Definition) string {
		return string(d.Type)
	})
}

func Types() []Type {
	defs := DefaultDefinitions()
	types := make([]Type, len(defs))
	for i, d := range defs {
		types[i] = d.Type
	}
	return types
}

func (t Type) Known() bool {
	switch t {
	case OrganizationCreated,
		OrganizationUpdated,
		OrganizationDeleted,
		MembershipCreated,
		MembershipUpdated,
		MembershipDeleted,
		WorkspaceCreated,
		WorkspaceUpdated,
		WorkspaceDeleted,
		OrganizationAPIKeyCreated,
		OrganizationAPIKeyUpdated,
		OrganizationAPIKeyDeleted,
		ProductUserCreated,
		ProductUserUpdated,
		ProductUserDeleted,
		OrganizationLicenseUpdated,
		ProductRoleCreated,
		ProductRoleUpdated,
		ProductRoleDeleted,
		ProductResourcePermissionCreated,
		ProductResourcePermissionUpdated,
		ProductResourcePermissionDeleted,
		ClerkUserCreated,
		ClerkUserUpdated,
		ClerkUserDeleted:
		return true
	default:
		return false
	}
}

func DefaultDefinitions() []Definition {
	return []Definition{
		// Organizations & Memberships
		{
			Type:        OrganizationCreated,
			Name:        "Organization created",
			Description: "Emitted when a new organization is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeOrganizations,
			Theme:       ThemeOrganizations,
		},
		{
			Type:        OrganizationUpdated,
			Name:        "Organization updated",
			Description: "Emitted when an organization's details are updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeOrganizations,
			Theme:       ThemeOrganizations,
		},
		{
			Type:        OrganizationDeleted,
			Name:        "Organization deleted",
			Description: "Emitted when an organization is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeOrganizations,
			Theme:       ThemeOrganizations,
		},
		{
			Type:        MembershipCreated,
			Name:        "Membership created",
			Description: "Emitted when a member is added to an organization.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeOrganizations,
			Theme:       ThemeOrganizations,
		},
		{
			Type:        MembershipUpdated,
			Name:        "Membership updated",
			Description: "Emitted when an organization member's role is updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeOrganizations,
			Theme:       ThemeOrganizations,
		},
		{
			Type:        MembershipDeleted,
			Name:        "Membership deleted",
			Description: "Emitted when an organization member is removed.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeOrganizations,
			Theme:       ThemeOrganizations,
		},

		// Workspaces
		{
			Type:        WorkspaceCreated,
			Name:        "Workspace created",
			Description: "Emitted when a workspace is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeWorkspaces,
			Theme:       ThemeWorkspaces,
		},
		{
			Type:        WorkspaceUpdated,
			Name:        "Workspace updated",
			Description: "Emitted when a workspace is updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeWorkspaces,
			Theme:       ThemeWorkspaces,
		},
		{
			Type:        WorkspaceDeleted,
			Name:        "Workspace deleted",
			Description: "Emitted when a workspace is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeWorkspaces,
			Theme:       ThemeWorkspaces,
		},

		// Organization API Keys
		{
			Type:        OrganizationAPIKeyCreated,
			Name:        "API key created",
			Description: "Emitted when an organization API key is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeAPIKeys,
			Theme:       ThemeAPIKeys,
		},
		{
			Type:        OrganizationAPIKeyUpdated,
			Name:        "API key updated",
			Description: "Emitted when an organization API key is updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeAPIKeys,
			Theme:       ThemeAPIKeys,
		},
		{
			Type:        OrganizationAPIKeyDeleted,
			Name:        "API key deleted",
			Description: "Emitted when an organization API key is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeAPIKeys,
			Theme:       ThemeAPIKeys,
		},

		// Product Users
		{
			Type:        ProductUserCreated,
			Name:        "Product user created",
			Description: "Emitted when a product user is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeUsers,
			Theme:       ThemeUsers,
		},
		{
			Type:        ProductUserUpdated,
			Name:        "Product user updated",
			Description: "Emitted when a product user is updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeUsers,
			Theme:       ThemeUsers,
		},
		{
			Type:        ProductUserDeleted,
			Name:        "Product user deleted",
			Description: "Emitted when a product user is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeUsers,
			Theme:       ThemeUsers,
		},

		// Licensing
		{
			Type:        OrganizationLicenseUpdated,
			Name:        "Organization license updated",
			Description: "Emitted when an organization license is instantiated, adjusted, or migrated.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeLicensing,
			Theme:       ThemeLicensing,
		},

		// Roles & Permissions
		{
			Type:        ProductRoleCreated,
			Name:        "Role created",
			Description: "Emitted when a product role is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeRolesPermissions,
			Theme:       ThemeRolesPermissions,
		},
		{
			Type:        ProductRoleUpdated,
			Name:        "Role updated",
			Description: "Emitted when a product role is updated or its permissions change.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeRolesPermissions,
			Theme:       ThemeRolesPermissions,
		},
		{
			Type:        ProductRoleDeleted,
			Name:        "Role deleted",
			Description: "Emitted when a product role is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeRolesPermissions,
			Theme:       ThemeRolesPermissions,
		},
		{
			Type:        ProductResourcePermissionCreated,
			Name:        "Resource permission created",
			Description: "Emitted when a resource permission is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeRolesPermissions,
			Theme:       ThemeRolesPermissions,
		},
		{
			Type:        ProductResourcePermissionUpdated,
			Name:        "Resource permission updated",
			Description: "Emitted when a resource permission is updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeRolesPermissions,
			Theme:       ThemeRolesPermissions,
		},
		{
			Type:        ProductResourcePermissionDeleted,
			Name:        "Resource permission deleted",
			Description: "Emitted when a resource permission is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   ThemeRolesPermissions,
			Theme:       ThemeRolesPermissions,
		},

		// Integrations (Clerk)
		{
			Type:        ClerkUserCreated,
			Name:        "Clerk user created",
			Description: "Emitted when a product user is created from a Clerk webhook.",
			GroupType:   GroupTypeIntegration,
			GroupName:   IntegrationClerk,
			Integration: IntegrationClerk,
		},
		{
			Type:        ClerkUserUpdated,
			Name:        "Clerk user updated",
			Description: "Emitted when a product user is updated from a Clerk webhook.",
			GroupType:   GroupTypeIntegration,
			GroupName:   IntegrationClerk,
			Integration: IntegrationClerk,
		},
		{
			Type:        ClerkUserDeleted,
			Name:        "Clerk user deleted",
			Description: "Emitted when a product user is deleted from a Clerk webhook.",
			GroupType:   GroupTypeIntegration,
			GroupName:   IntegrationClerk,
			Integration: IntegrationClerk,
		},
	}
}
