// Package authz will decide access with OpenFGA (ADR-0021, ADR-0022 E3 and
// E5), running inside Anchor on Anchor's Postgres. Anchor's tables stay the
// source of truth; each Product's OpenFGA store mirrors them. For now the
// engine runs and each Product can get its store; nothing reads them yet.
package authz

import (
	"go.uber.org/fx"

	"anchor/internal/authz/engine"
	"anchor/internal/authz/store"
)

func NewModule() fx.Option {
	return fx.Module(
		"authz",
		fx.Provide(
			engine.New,
			store.New,
		),
		// The engine starts with the application, in the background, so its
		// migrations and datastore are proven in every environment before
		// anything reads it, and a failure there never stops Anchor serving.
		fx.Invoke(func(engine.Provider) {}),
	)
}
