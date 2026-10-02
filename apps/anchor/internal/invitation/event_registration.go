package invitation

import "anchor/internal/events"

func EventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupOrganizations,
		events.Definition{
			Type:        events.OrganizationInvitationCreated,
			Name:        "Invitation created",
			Description: "Emitted when an organization invitation is created.",
		},
		events.Definition{
			Type:        events.OrganizationInvitationUpdated,
			Name:        "Invitation updated",
			Description: "Emitted when an organization invitation is updated.",
		},
		events.Definition{
			Type:        events.OrganizationInvitationDeleted,
			Name:        "Invitation deleted",
			Description: "Emitted when an organization invitation is deleted.",
		},
		events.Definition{
			Type:        events.OrganizationInvitationAccepted,
			Name:        "Invitation accepted",
			Description: "Emitted when an organization invitation is accepted. The membership created event follows.",
		},
	)
}
