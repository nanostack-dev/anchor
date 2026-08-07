package license

import (
	"anchor/internal/license/repository"
	"anchor/internal/license/service"

	"go.uber.org/fx"
)

// NewModule wires the licensing subsystem: the per-Product license schema, the
// license templates declared against it, and one Organization's own copy of a
// template's values.
func NewModule() fx.Option {
	return fx.Module(
		"license",
		fx.Provide(
			repository.NewSchemaRepository,
			repository.NewSchemaFieldRepository,
			repository.NewTemplateRepository,
			repository.NewOrganizationLicenseRepository,
			service.NewLicenseSchemaService,
			service.NewLicenseTemplateService,
			service.NewOrganizationLicenseService,
		),
	)
}
