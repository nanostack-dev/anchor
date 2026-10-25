package app

import (
	"fmt"

	"anchor/internal/access"

	"github.com/nanostack-dev/nanostack-framework/modules/cache"
	"github.com/nanostack-dev/nanostack-framework/modules/config"
	"github.com/nanostack-dev/nanostack-framework/modules/logging"
	"github.com/nanostack-dev/nanostack-framework/modules/migrations"
	"github.com/nanostack-dev/nanostack-framework/modules/pglock"
	"github.com/nanostack-dev/nanostack-framework/modules/postgres"
	"github.com/nanostack-dev/nanostack-framework/modules/tracing"
	"github.com/nanostack-dev/nanostack-framework/modules/transactor"

	httpserver "anchor/cmd/http"
	"anchor/internal/api"
	"anchor/internal/authz"
	"anchor/internal/email"
	"anchor/internal/events"
	"anchor/internal/integration"
	"anchor/internal/invitation"
	"anchor/internal/license"
	"anchor/internal/mapper"
	"anchor/internal/middleware"
	"anchor/internal/organizationrole"
	"anchor/internal/repository"
	"anchor/internal/resourcetype"
	"anchor/internal/runtimeenv"
	"anchor/internal/service"
	"anchor/internal/session"

	"go.uber.org/fx"
)

func StartAnchor() {
	startAnchor()
}

func StartAnchorWithPopulate(target ...any) {
	startAnchor(target...)
}

func startAnchor(target ...any) {
	if err := runtimeenv.HydrateFileBackedEnv(); err != nil {
		panic(fmt.Sprintf("failed to hydrate file-backed runtime env: %v", err))
	}

	app := fx.New(
		logging.Module,
		tracing.Module,
		config.Module,
		postgres.Module,
		transactor.Module,
		cache.Module,
		migrations.Module,
		pglock.Module,
		mapper.NewModule(),
		repository.NewModule(),
		resourcetype.NewModule(),
		organizationrole.NewModule(),
		authz.NewModule(),
		authz.ShadowDecisions(),
		service.NewModule(),
		integration.NewModule(),
		email.NewModule(),
		events.NewModule(),
		license.NewModule(),
		invitation.NewModule(),
		access.NewModule(),
		session.NewModule(),
		api.NewModule(),
		middleware.NewModule(),
		httpserver.NewHTTPServerModule(),
		fx.Populate(target...),
	)
	app.Run()
	if err := app.Err(); err != nil {
		return
	}
}
