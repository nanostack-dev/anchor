// Package roleassignment wires the roles members hold on registered resources
// (ADR-0022 D3, D4).
package roleassignment

import (
	"go.uber.org/fx"

	"anchor/internal/events"
	"anchor/internal/roleassignment/repository"
	"anchor/internal/roleassignment/service"
)

func NewModule() fx.Option {
	return fx.Module(
		"role_assignment",
		fx.Provide(
			repository.NewRepository,
			service.NewService,
			events.AsRegistration(EventRegistration),
		),
	)
}
