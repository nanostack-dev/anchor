package license

import "anchor/internal/events"

func EventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupLicensing,
		events.Definition{
			Type:        events.OrganizationLicenseUpdated,
			Name:        "Organization license updated",
			Description: "Emitted when an organization license is instantiated, adjusted, or migrated.",
		},
	)
}
