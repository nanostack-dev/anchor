package session

import (
	"anchor/internal/session/repository"
	"anchor/internal/session/service"

	"go.uber.org/fx"
)

func NewModule() fx.Option {
	return fx.Module(
		"session",
		fx.Provide(repository.NewRepository, service.NewService),
	)
}
