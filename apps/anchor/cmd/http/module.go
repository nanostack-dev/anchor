package httpserver

import (
	_ "embed"

	apisecmodule "github.com/nanostack-dev/nanostack-framework/modules/apisec"
	"github.com/nanostack-dev/nanostack-framework/modules/config"

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
		// The auth middleware resolves each operation's security requirements
		// from this document; the framework module owns building that resolver.
		fx.Supply(apisecmodule.Document(OpenAPI)),
		apisecmodule.NewModule(),
		fx.Invoke(
			RegisterServer,
		),
	)
}
