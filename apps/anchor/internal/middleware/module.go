package middleware

import (
	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

var Module = nano.Module("middleware", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		NewAuthMiddleware,
	),
)
