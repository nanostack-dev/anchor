package applog

import (
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module provides the loggers: Loggers, the component *slog.Logger, Scoped
// and Identity. Building it installs the scoped logger as the slog default.
//
// It stands where core/logging.Module would: core/logging builds one logger,
// with fractional seconds and the request scope on every line, and anchor's
// line shape needs the two loggers of New (see the package comment).
var Module = nano.Module("applog", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(NewLoggers, ComponentLogger, ScopedLogger, IdentityAttrs),
)

// ComponentLogger is the logger injected into services, repositories and
// handlers.
func ComponentLogger(l Loggers) *slog.Logger { return l.Component }

// ScopedLogger is the logger of the HTTP boundary.
func ScopedLogger(l Loggers) Scoped { return l.Scoped }

// IdentityAttrs is the service identity that auth adds to the request scope.
func IdentityAttrs(l Loggers) Identity { return l.Identity }
