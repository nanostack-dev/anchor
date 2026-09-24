package license

import "anchor/internal/events"

func EventRegistration() events.Registration {
	return events.RegisterDomain(
		events.ThemeLicensing,
		events.Definition{
			Type:        events.OrganizationLicenseUpdated,
			Name:        "Organization license updated",
			Description: "Emitted when an organization license is instantiated, adjusted, or migrated.",
		},
	)
}
