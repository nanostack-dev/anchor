package license

import (
	"anchor/internal/license/repository"
	"anchor/internal/license/service"

	"go.uber.org/fx"
)

// NewModule wires the licensing subsystem: the per-Product license schema, the
// license templates declared against it, one Organization's own copy of a
// template's values, every change ever made to that copy, moving a set of
// Organizations onto another template, and what the Organization has used
// against it.
func NewModule() fx.Option {
	return fx.Module(
		"license",
		fx.Provide(
			repository.NewSchemaRepository,
			repository.NewSchemaFieldRepository,
			repository.NewTemplateRepository,
			repository.NewOrganizationLicenseRepository,
			repository.NewOrganizationLicenseChangeRepository,
			repository.NewUsageObservationRepository,
			repository.NewUsageSeriesRepository,
			service.NewLicenseSchemaService,
			service.NewLicenseTemplateService,
			service.NewOrganizationLicenseService,
			service.NewLicenseHistoryService,
			service.NewLicenseMigrationService,
			service.NewUsageService,
			service.NewUsageSeriesService,
		),
	)
}
