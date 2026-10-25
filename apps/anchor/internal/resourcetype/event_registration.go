package resourcetype

import "anchor/internal/events"

func EventRegistration() events.Registration {
	return events.RegisterInternal(
		events.GroupRolesPermissions,
		events.Definition{
			Type:        events.ProductResourceTypeCreated,
			Name:        "Resource type created",
			Description: "Emitted when a resource type is added to the product's tree.",
		},
		events.Definition{
			Type:        events.ProductResourceTypeUpdated,
			Name:        "Resource type updated",
			Description: "Emitted when a resource type is renamed, or its parent types or nesting change.",
		},
		events.Definition{
			Type:        events.ProductResourceTypeDeleted,
			Name:        "Resource type deleted",
			Description: "Emitted when a resource type is removed from the product's tree.",
		},
	)
}
