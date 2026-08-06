package license

import (
	"anchor/internal/license/repository"
	"anchor/internal/license/service"

	"go.uber.org/fx"
)

// NewModule wires the licensing subsystem (license schema declaration).
func NewModule() fx.Option {
	return fx.Module(
		"license",
		fx.Provide(
			repository.NewSchemaRepository,
			repository.NewSchemaFieldRepository,
			service.NewLicenseService,
		),
	)
}
