package roleassignment

import "anchor/internal/events"

// EventRegistration catalogs the role assignment events. Unlike resource
// events they reach the product's endpoint (ADR-0022 G3).
func EventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupResources,
		events.Definition{
			Type:        events.ResourceRoleAssignmentCreated,
			Name:        "Resource role assigned",
			Description: "Emitted when a member gets a role on a resource where they held none.",
		},
		events.Definition{
			Type:        events.ResourceRoleAssignmentUpdated,
			Name:        "Resource role changed",
			Description: "Emitted when a member's role on a resource is replaced.",
		},
		events.Definition{
			Type:        events.ResourceRoleAssignmentDeleted,
			Name:        "Resource role removed",
			Description: "Emitted for each role a member stops holding on a resource: removed, left the organization, or the resource deleted.",
		},
	)
}
