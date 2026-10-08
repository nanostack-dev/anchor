package stripebilling

import "go.uber.org/fx"

func NativeModule() fx.Option {
	return fx.Module(
		"stripe-billing",
		fx.Provide(NewManager),
		fx.Invoke(RegisterWorker),
	)
}
