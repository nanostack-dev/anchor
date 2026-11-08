package license

import (
	"anchor/internal/events"
	"anchor/internal/license/repository"
	"anchor/internal/license/service"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module wires the licensing subsystem: the per-Product license schema, the
// license templates declared against it, one Organization's own copy of a
// template's values, every change ever made to that copy, moving a set of
// Organizations onto another template, propagating a template value update
// onto the licenses that follow it, and what the Organization has used
// against it.
var Module = nano.Module("license", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		repository.NewSchemaRepository,
		repository.NewSchemaFieldRepository,
		repository.NewTemplateRepository,
		repository.NewOrganizationLicenseRepository,
		repository.NewOrganizationLicenseChangeRepository,
		repository.NewUsageObservationRepository,
		repository.NewUsageSeriesRepository,
		service.NewLicenseSchemaService,
		service.NewLicenseTemplateService,
		service.NewLicenseTemplateSyncEnqueuer,
		service.NewOrganizationLicenseEntries,
		service.NewLicenseTemplateSyncService,
		service.NewOrganizationLicenseService,
		service.NewLicenseHistoryService,
		service.NewLicenseMigrationService,
		service.NewUsageService,
		service.NewUsageSeriesService,
		service.NewLicenseTemplateSyncWorker,
	),
	nano.Collect[events.Registration](EventRegistration),
)
