package httpserver

import (
	_ "embed"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
	"github.com/nanostack-dev/nanostack-framework/http/nanoapisec"
	"github.com/nanostack-dev/nanostack-framework/http/nanoserver"
	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"
)

//go:embed openapi.yaml
var OpenAPI []byte

// Module serves anchor's router with nanoserver and runs the pgkit admin UI.
var Module = nano.Module("httpserver", //nolint:gochecknoglobals // A nano module is a package-level value.
	nanoserver.Module,
	nano.Providers(ListenerConfig, NewResolver, NewHandler, NewQueueDashboard),
)

// NewResolver builds the security-requirement resolver of the auth
// middleware from the embedded contract. It matches paths as chi routes them,
// as modules/apisec did; nanoapisec.Module would match them as
// http.ServeMux does.
func NewResolver() (*apisec.Resolver, error) {
	return nanoapisec.NewResolverWith(nanoapisec.Document(OpenAPI), apisec.MatchChi())
}
