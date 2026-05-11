package app

import (
	"github.com/nanostack-dev/shared/fxmodules/cache"
	"github.com/nanostack-dev/shared/fxmodules/config"
	"github.com/nanostack-dev/shared/fxmodules/database/postgres"
	"github.com/nanostack-dev/shared/fxmodules/logging"
	"github.com/nanostack-dev/shared/fxmodules/migrations"
	"github.com/nanostack-dev/shared/fxmodules/pglock"
	sharedsentry "github.com/nanostack-dev/shared/fxmodules/sentry"

	httpserver "anchor/cmd/http"
	"anchor/internal/api"
	"anchor/internal/email"
	"anchor/internal/integration"
	"anchor/internal/mapper"
	"anchor/internal/middleware"
	"anchor/internal/repository"
	"anchor/internal/service"

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
