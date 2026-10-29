package resource

import "anchor/internal/events"

// EventRegistration catalogs the resource events. They are frequent, one per
// registered folder or share, so they stay inside Anchor (ADR-0022 G3): the
// relationship store follows them, and no product endpoint receives them.
func EventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupResources,
		events.Definition{
			Type:        events.ResourceCreated,
			Name:        "Resource registered",
			Description: "Emitted when a product registers a resource in an organization.",
			Undelivered: true,
		},
		events.Definition{
			Type:        events.ResourceUpdated,
			Name:        "Resource updated",
			Description: "Emitted when a registered resource is renamed or moved.",
			Undelivered: true,
		},
		events.Definition{
			Type:        events.ResourceDeleted,
			Name:        "Resource deleted",
			Description: "Emitted for each registered resource a delete removes.",
			Undelivered: true,
		},
	)
}
