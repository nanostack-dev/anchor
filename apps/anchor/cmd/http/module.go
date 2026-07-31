package httpserver

import (
	_ "embed"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/nanostack-dev/nanostack-framework/modules/config"
	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"

	"go.uber.org/fx"
)

//go:embed openapi.yaml
var OpenAPI []byte

func NewHTTPServerModule() fx.Option {
	return fx.Module(
		"httpserver",
		fx.Provide(
			func(loader config.Loader) (*ServerConfig, error) {
				var config ServerConfig
				loader.MustLoadConfig("server", &config)
				return &config, nil
			},
			// The auth middleware reads each operation's security requirements
			// from the contract rather than from generated context keys, so the
			// route index is built once here, at startup, from the same
			// embedded document the request validator uses. A malformed
			// document fails startup instead of silently leaving routes
			// unguarded.
			func() (*apisec.Resolver, error) {
				doc, err := openapi3.NewLoader().LoadFromData(OpenAPI)
				if err != nil {
					return nil, fmt.Errorf("load embedded OpenAPI document: %w", err)
				}
				return apisec.NewResolver(doc)
			},
		),
		fx.Invoke(
			RegisterServer,
		),
	)
}
