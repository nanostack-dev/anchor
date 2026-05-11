package api

import (
	"net/http"

	"go.uber.org/fx"
)

func NewModule() fx.Option {
	return fx.Module(
		"http",
		fx.Provide(
			NewAPI,
		),
	)
}

type AuthMiddlewareResult struct {
	Middleware func(http.Handler) http.Handler
}
