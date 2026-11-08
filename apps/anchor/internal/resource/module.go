package resource

import (
	"anchor/internal/events"
	"anchor/internal/resource/repository"
	"anchor/internal/resource/service"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

var Module = nano.Module("resource", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		repository.NewRepository,
		service.NewService,
	),
	nano.Collect[events.Registration](EventRegistration),
)
