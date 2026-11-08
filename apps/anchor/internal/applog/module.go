package applog

import (
	"log/slog"

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
