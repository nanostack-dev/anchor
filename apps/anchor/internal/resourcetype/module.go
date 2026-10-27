// Package resourcetype wires a product's tree of resource types (ADR-0022).
package resourcetype

import (
	"go.uber.org/fx"

	"anchor/internal/events"
	"anchor/internal/resourcetype/repository"
	"anchor/internal/resourcetype/service"
)

func NewModule() fx.Option {
	return fx.Module(
		"resource_type",
		fx.Provide(
			repository.NewRepository,
			service.NewService,
			events.AsRegistration(EventRegistration),
		),
		fx.Invoke(RegisterSchemaCheck),
	)
}
