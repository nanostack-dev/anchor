package app

import (
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/modules/cache"
	"github.com/nanostack-dev/nanostack-framework/modules/config"
	"github.com/nanostack-dev/nanostack-framework/modules/logging"
	"github.com/nanostack-dev/nanostack-framework/modules/migrations"
	"github.com/nanostack-dev/nanostack-framework/modules/pglock"
	"github.com/nanostack-dev/nanostack-framework/modules/postgres"
	"github.com/nanostack-dev/nanostack-framework/modules/transactor"

	httpserver "anchor/cmd/http"
	"anchor/internal/api"
	"anchor/internal/email"
	"anchor/internal/integration"
	"anchor/internal/mapper"
	"anchor/internal/middleware"
	"anchor/internal/repository"
	"anchor/internal/runtimeenv"
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
	if err := runtimeenv.HydrateFileBackedEnv(); err != nil {
		panic(fmt.Sprintf("failed to hydrate file-backed runtime env: %v", err))
	}

	app := fx.New(
		logging.Module,
		config.Module,
		postgres.Module,
		transactor.Module,
		cache.Module,
		migrations.Module,
		pglock.Module,
		mapper.NewModule(),
		repository.NewModule(),
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
