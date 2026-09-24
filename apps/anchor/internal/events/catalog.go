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

type IntegrationRegistration struct {
	ProviderType string
	Events       []provider.WebhookEvent
}

func RegisterIntegration(providerType string, integrationEvents ...provider.WebhookEvent) IntegrationRegistration {
	return IntegrationRegistration{ProviderType: providerType, Events: integrationEvents}
}

type Catalog interface {
	All() []Definition
	Types() []Type
	IsKnown(t Type) bool
	AllEventTypesStrings() []string
}

type CatalogParams struct {
	fx.In
	DomainRegistrations      []DomainRegistration      `group:"domain_events"`
	IntegrationRegistrations []IntegrationRegistration `group:"integration_events"`
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
	for _, reg := range p.IntegrationRegistrations {
		for _, ev := range reg.Events {
			defs = append(defs, Definition{
				Type:        Type(ev.Type),
				Name:        ev.Name,
				Description: ev.Description,
				GroupType:   GroupTypeIntegration,
				GroupName:   reg.ProviderType,
				Integration: reg.ProviderType,
			})
		}
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
