package applog

import (
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/modules/logging"
	"go.uber.org/fx"
)

// Module provides the loggers: Loggers, the component *slog.Logger, Scoped
// and Identity. Building it installs the scoped logger as the slog default.
func Module() fx.Option {
	return fx.Module(
		"applog",
		fx.Provide(
			NewFromEnv,
			func(l Loggers) *slog.Logger { return l.Component },
			func(l Loggers) Scoped { return l.Scoped },
			func(l Loggers) Identity { return l.Identity },
		),
	)
}

// LegacyZerolog provides the zerolog.Logger that the deprecated framework fx
// modules (tracing, config, postgres, cache, migrations, pglock) and
// modules/cache's cache.New still take, as modules/logging built it, without
// the pkg/log Binder: anchor's own code logs through Module's loggers. Delete
// it when the last of those modules moves to its nano replacement.
func LegacyZerolog() fx.Option {
	return fx.Provide(logging.NewLoggingConfig, logging.NewZerologLogger)
}
