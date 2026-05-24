package transactor

import (
	"go.uber.org/fx"
)

func NewModule() fx.Option {
	return fx.Module(
		"transactor",
		fx.Provide(
			New,
		),
	)
}
