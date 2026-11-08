package resourcetype

import (
	"anchor/internal/events"
	"anchor/internal/resourcetype/repository"
	"anchor/internal/resourcetype/service"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

var Module = nano.Module("resource_type", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		repository.NewRepository,
		service.NewService,
		NewSchemaCheck,
	),
	nano.Collect[events.Registration](EventRegistration),
)
