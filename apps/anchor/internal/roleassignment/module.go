// Package roleassignment wires the roles members hold on registered resources
// (ADR-0022 D3, D4).
package roleassignment

import (
	"anchor/internal/events"
	"anchor/internal/roleassignment/repository"
	"anchor/internal/roleassignment/service"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

var Module = nano.Module("role_assignment", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		repository.NewRepository,
		service.NewService,
	),
	nano.Collect[events.Registration](EventRegistration),
)
