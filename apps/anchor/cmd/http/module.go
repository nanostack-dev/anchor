package httpserver

import (
	_ "embed"

	"github.com/nanostack-dev/shared/fxmodules/config"

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
		),
		fx.Invoke(
			RegisterServer,
		),
	)
}
