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

var defaultCatalogInstance = NewCatalog(CatalogParams{})

func Types() []Type {
	return defaultCatalogInstance.Types()
}

func (t Type) Known() bool {
	return defaultCatalogInstance.IsKnown(t)
}

func DefaultDefinitions() []Definition {
	return []Definition{
		// Organizations & Memberships
		{
			Type:        OrganizationCreated,
			Name:        "Organization created",
			Description: "Emitted when a new organization is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Organizations",
			Theme:       "Organizations",
		},
		{
			Type:        OrganizationUpdated,
			Name:        "Organization updated",
			Description: "Emitted when an organization's details are updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Organizations",
			Theme:       "Organizations",
		},
		{
			Type:        OrganizationDeleted,
			Name:        "Organization deleted",
			Description: "Emitted when an organization is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Organizations",
			Theme:       "Organizations",
		},
		{
			Type:        MembershipCreated,
			Name:        "Membership created",
			Description: "Emitted when a member is added to an organization.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Organizations",
			Theme:       "Organizations",
		},
		{
			Type:        MembershipUpdated,
			Name:        "Membership updated",
			Description: "Emitted when an organization member's role is updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Organizations",
			Theme:       "Organizations",
		},
		{
			Type:        MembershipDeleted,
			Name:        "Membership deleted",
			Description: "Emitted when an organization member is removed.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Organizations",
			Theme:       "Organizations",
		},

		// Workspaces
		{
			Type:        WorkspaceCreated,
			Name:        "Workspace created",
			Description: "Emitted when a workspace is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Workspaces",
			Theme:       "Workspaces",
		},
		{
			Type:        WorkspaceUpdated,
			Name:        "Workspace updated",
			Description: "Emitted when a workspace is updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Workspaces",
			Theme:       "Workspaces",
		},
		{
			Type:        WorkspaceDeleted,
			Name:        "Workspace deleted",
			Description: "Emitted when a workspace is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Workspaces",
			Theme:       "Workspaces",
		},

		// Organization API Keys
		{
			Type:        OrganizationAPIKeyCreated,
			Name:        "API key created",
			Description: "Emitted when an organization API key is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   "API Keys",
			Theme:       "API Keys",
		},
		{
			Type:        OrganizationAPIKeyUpdated,
			Name:        "API key updated",
			Description: "Emitted when an organization API key is updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   "API Keys",
			Theme:       "API Keys",
		},
		{
			Type:        OrganizationAPIKeyDeleted,
			Name:        "API key deleted",
			Description: "Emitted when an organization API key is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   "API Keys",
			Theme:       "API Keys",
		},

		// Product Users
		{
			Type:        ProductUserCreated,
			Name:        "Product user created",
			Description: "Emitted when a product user is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Users",
			Theme:       "Users",
		},
		{
			Type:        ProductUserUpdated,
			Name:        "Product user updated",
			Description: "Emitted when a product user is updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Users",
			Theme:       "Users",
		},
		{
			Type:        ProductUserDeleted,
			Name:        "Product user deleted",
			Description: "Emitted when a product user is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Users",
			Theme:       "Users",
		},

		// Licensing
		{
			Type:        OrganizationLicenseUpdated,
			Name:        "Organization license updated",
			Description: "Emitted when an organization license is instantiated, adjusted, or migrated.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Licensing",
			Theme:       "Licensing",
		},

		// Roles & Permissions
		{
			Type:        ProductRoleCreated,
			Name:        "Role created",
			Description: "Emitted when a product role is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Roles & Permissions",
			Theme:       "Roles & Permissions",
		},
		{
			Type:        ProductRoleUpdated,
			Name:        "Role updated",
			Description: "Emitted when a product role is updated or its permissions change.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Roles & Permissions",
			Theme:       "Roles & Permissions",
		},
		{
			Type:        ProductRoleDeleted,
			Name:        "Role deleted",
			Description: "Emitted when a product role is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Roles & Permissions",
			Theme:       "Roles & Permissions",
		},
		{
			Type:        ProductResourcePermissionCreated,
			Name:        "Resource permission created",
			Description: "Emitted when a resource permission is created.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Roles & Permissions",
			Theme:       "Roles & Permissions",
		},
		{
			Type:        ProductResourcePermissionUpdated,
			Name:        "Resource permission updated",
			Description: "Emitted when a resource permission is updated.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Roles & Permissions",
			Theme:       "Roles & Permissions",
		},
		{
			Type:        ProductResourcePermissionDeleted,
			Name:        "Resource permission deleted",
			Description: "Emitted when a resource permission is deleted.",
			GroupType:   GroupTypeTheme,
			GroupName:   "Roles & Permissions",
			Theme:       "Roles & Permissions",
		},

		// Integrations (Clerk)
		{
			Type:        ClerkUserCreated,
			Name:        "Clerk user created",
			Description: "Emitted when a product user is created from a Clerk webhook.",
			GroupType:   GroupTypeIntegration,
			GroupName:   "CLERK",
			Integration: "CLERK",
		},
		{
			Type:        ClerkUserUpdated,
			Name:        "Clerk user updated",
			Description: "Emitted when a product user is updated from a Clerk webhook.",
			GroupType:   GroupTypeIntegration,
			GroupName:   "CLERK",
			Integration: "CLERK",
		},
		{
			Type:        ClerkUserDeleted,
			Name:        "Clerk user deleted",
			Description: "Emitted when a product user is deleted from a Clerk webhook.",
			GroupType:   GroupTypeIntegration,
			GroupName:   "CLERK",
			Integration: "CLERK",
		},
	}
}
