package session

import (
	"anchor/internal/session/repository"

	"go.uber.org/fx"
)

// NewModule wires the platform user session store: one row per login, and the
// rotating refresh tokens that keep it alive.
func NewModule() fx.Option {
	return fx.Module(
		"platform_session",
		fx.Provide(repository.NewRepository),
	)
}
