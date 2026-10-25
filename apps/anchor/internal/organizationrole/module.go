// Package organizationrole wires the roles each Organization owns: a copy of
// every role template of its Product (ADR-0022 D8).
package organizationrole

import (
	"go.uber.org/fx"

	"anchor/internal/organizationrole/repository"
	"anchor/internal/organizationrole/service"
)

func NewModule() fx.Option {
	return fx.Module(
		"organization_role",
		fx.Provide(
			repository.NewRepository,
			service.NewService,
		),
		fx.Invoke(service.RegisterKeyBackfill),
	)
}
