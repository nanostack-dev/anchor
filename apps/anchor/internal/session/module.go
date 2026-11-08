package session

import (
	"anchor/internal/session/repository"
	"anchor/internal/session/service"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

var Module = nano.Module("session", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		repository.NewRepository,
		service.NewService,
	),
)
