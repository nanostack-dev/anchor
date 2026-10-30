// Package resource wires the registry of a product's own resources
// (ADR-0022 B1-B4).
package resource

import (
	"go.uber.org/fx"

	"anchor/internal/events"
	"anchor/internal/resource/repository"
	"anchor/internal/resource/service"
)

func NewModule() fx.Option {
	return fx.Module(
		"resource",
		fx.Provide(
			repository.NewRepository,
			service.NewService,
			events.AsRegistration(EventRegistration),
		),
	)
}
