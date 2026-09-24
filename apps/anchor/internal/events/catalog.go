package events

import (
	"slices"

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
)

type Definition struct {
	Type        Type      `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	GroupType   GroupType `json:"group_type"`
	GroupName   string    `json:"group_name"`
}

type Registration struct {
	GroupType   GroupType
	GroupName   string
	Definitions []Definition
}

func RegisterDomain(theme string, definitions ...Definition) Registration {
	return Registration{GroupType: GroupTypeTheme, GroupName: theme, Definitions: definitions}
}

func RegisterIntegration(providerType string, definitions ...Definition) Registration {
	return Registration{GroupType: GroupTypeIntegration, GroupName: providerType, Definitions: definitions}
}

func AsRegistration(fn any) any {
	return fx.Annotate(fn, fx.ResultTags(`group:"product_events"`))
}

type Catalog interface {
	All() []Definition
	IsKnown(t Type) bool
	AllEventTypesStrings() []string
}

type CatalogParams struct {
	fx.In
	Registrations []Registration `group:"product_events"`
}

type catalog struct {
	definitions []Definition
	byType      map[Type]Definition
}

func NewCatalog(p CatalogParams) Catalog {
	var defs []Definition
	for _, reg := range p.Registrations {
		for _, definition := range reg.Definitions {
			definition.GroupType = reg.GroupType
			definition.GroupName = reg.GroupName
			defs = append(defs, definition)
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

func (c *catalog) IsKnown(t Type) bool {
	_, ok := c.byType[t]
	return ok
}

func (c *catalog) AllEventTypesStrings() []string {
	return functional.Slice(c.definitions).Map(func(d Definition) string {
		return string(d.Type)
	})
}
