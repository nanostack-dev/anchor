package api

import (
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

var Module = nano.Module("http", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(
		NewAPI,
	),
)

type AuthMiddlewareResult struct {
	Middleware func(http.Handler) http.Handler
}
