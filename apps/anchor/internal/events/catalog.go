package events

import (
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"go.uber.org/fx"
)

type GroupType string

const (
	GroupTypeInternal    GroupType = "internal"
	GroupTypeIntegration GroupType = "integration"
)

const (
	GroupOrganizations    = "Organizations"
	GroupWorkspaces       = "Workspaces"
	GroupAPIKeys          = "API Keys"
	GroupUsers            = "Users"
	GroupLicensing        = "Licensing"
	GroupRolesPermissions = "Roles & Permissions"
	GroupResources        = "Resources"
)

type Definition struct {
	Type        Type      `json:"type"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	GroupType   GroupType `json:"group_type"`
	GroupName   string    `json:"group_name"`
	// Undelivered keeps the event inside Anchor: it still reaches the
	// transaction observers, but it is never queued for delivery, listed in
	// the product's catalog, or accepted in an endpoint's subscription.
	Undelivered bool `json:"-"`
}

type Registration struct {
	GroupType   GroupType
	GroupName   string
	Definitions []Definition
}

func RegisterInternal(groupName string, definitions ...Definition) Registration {
	return Registration{GroupType: GroupTypeInternal, GroupName: groupName, Definitions: definitions}
}

func RegisterIntegration(providerType string, definitions ...Definition) Registration {
	return Registration{GroupType: GroupTypeIntegration, GroupName: providerType, Definitions: definitions}
}

func AsRegistration(fn any) any {
	return fx.Annotate(fn, fx.ResultTags(`group:"product_events"`))
}

type Catalog interface {
	// All lists the events a product endpoint may receive.
	All() []Definition
	// IsKnown reports whether the event may be emitted.
	IsKnown(t Type) bool
	// IsDeliverable reports whether the event may reach a product endpoint.
	IsDeliverable(t Type) bool
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
	return functional.Slice(c.definitions).Filter(func(d Definition) bool { return !d.Undelivered })
}

func (c *catalog) IsKnown(t Type) bool {
	_, ok := c.byType[t]
	return ok
}

func (c *catalog) IsDeliverable(t Type) bool {
	d, ok := c.byType[t]
	return ok && !d.Undelivered
}

func (c *catalog) AllEventTypesStrings() []string {
	return functional.Slice(c.All()).Map(func(d Definition) string {
		return string(d.Type)
	})
}
