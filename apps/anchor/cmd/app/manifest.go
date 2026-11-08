//go:build nanogen

package app

import (
	"github.com/nanostack-dev/nanostack-framework/core/nano"
	"github.com/nanostack-dev/nanostack-framework/otel/nanotrace"
	"github.com/nanostack-dev/nanostack-framework/pg/nanomigrate"
	"github.com/nanostack-dev/nanostack-framework/pg/nanopg"
	"github.com/nanostack-dev/nanostack-framework/redis/nanocache"

	httpserver "anchor/cmd/http"
	"anchor/internal/access"
	"anchor/internal/api"
	"anchor/internal/applog"
	"anchor/internal/authz"
	"anchor/internal/email"
	"anchor/internal/events"
	"anchor/internal/integration"
	"anchor/internal/invitation"
	"anchor/internal/license"
	"anchor/internal/mapper"
	"anchor/internal/middleware"
	"anchor/internal/repository"
	"anchor/internal/resource"
	"anchor/internal/resourcetype"
	"anchor/internal/roleassignment"
	"anchor/internal/service"
	"anchor/internal/session"
)

// App is anchor's manifest: nano gen wire reads it and writes wire.gen.go.
var App = nano.Manifest("anchor",
	applog.Module,
	nanotrace.Module,
	nanopg.Module,
	nanomigrate.Module,
	nanocache.Module,
	mapper.Module,
	repository.Module,
	resourcetype.Module,
	resource.Module,
	roleassignment.Module,
	authz.Module,
	service.Module,
	integration.Module,
	email.Module,
	events.Module,
	license.Module,
	invitation.Module,
	access.Module,
	session.Module,
	api.Module,
	middleware.Module,
	httpserver.Module,
)
