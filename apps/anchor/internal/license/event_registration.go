package license

import (
	"anchor/internal/events"

	"go.uber.org/fx"
)

func LicenseEventRegistration() events.DomainRegistration {
	return events.RegisterDomain(
		events.Definition{
			Type:        events.OrganizationLicenseUpdated,
			Name:        "Organization license updated",
			Description: "Emitted when an organization license is instantiated, adjusted, or migrated.",
			GroupType:   events.GroupTypeTheme,
			GroupName:   "Licensing",
			Theme:       "Licensing",
		},
	)
}

func AsDomainEventRegistration(fn any) any {
	return fx.Annotate(
		fn,
		fx.ResultTags(`group:"domain_events"`),
	)
}
