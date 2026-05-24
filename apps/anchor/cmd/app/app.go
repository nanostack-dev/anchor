package app

import (
	"github.com/nanostack-dev/nanostack-framework/modules/cache"
	"github.com/nanostack-dev/nanostack-framework/modules/config"
	"github.com/nanostack-dev/nanostack-framework/modules/logging"
	"github.com/nanostack-dev/nanostack-framework/modules/migrations"
	"github.com/nanostack-dev/nanostack-framework/modules/pglock"
	"github.com/nanostack-dev/nanostack-framework/modules/postgres"
	sharedsentry "github.com/nanostack-dev/nanostack-framework/modules/sentry"

	httpserver "anchor/cmd/http"
	"anchor/internal/api"
	"anchor/internal/email"
	"anchor/internal/integration"
	"anchor/internal/mapper"
	"anchor/internal/middleware"
	"anchor/internal/repository"
	"anchor/internal/service"
	"anchor/internal/transactor"

	"go.uber.org/fx"
)

type StartOptions struct {
	LocalTunnel bool
}

func StartAnchor() {
	StartAnchorWithOptions(StartOptions{})
}

func StartAnchorWithPopulate(target ...interface{}) {
	StartAnchorWithOptions(StartOptions{}, target...)
}

func StartAnchorWithOptions(options StartOptions, target ...interface{}) {
	app := fx.New(
		logging.Module,
		config.Module,
		postgres.Module,
		transactor.NewModule(),
		cache.Module,
		migrations.Module,
		pglock.Module,
		mapper.NewModule(),
		repository.NewModule(),
		sharedsentry.NewModule(),
		service.NewModule(),
		integration.NewModule(),
		email.NewModule(),
		api.NewModule(),
		middleware.NewModule(),
		newLocalTunnelModule(),
		fx.Supply(localTunnelConfig{Enabled: options.LocalTunnel}),
		httpserver.NewHTTPServerModule(),
		fx.Populate(target...),
	)
	app.Run()
	if err := app.Err(); err != nil {
		return
	}
}
