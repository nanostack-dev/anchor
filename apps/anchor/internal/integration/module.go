package integration

import (
	"anchor/internal/integration/provider"
	"anchor/internal/integration/provider/clerk"
	"anchor/internal/integration/provider/smtp"
	"anchor/internal/security/encryption"

	"go.uber.org/fx"
)

// NewModule wires the integration subsystem: encryption, provider registry,
// and all registered integration providers.
func NewModule() fx.Option {
	return fx.Module(
		"integration",
		encryption.NewModule(),
		provider.NewModule(),
		clerk.NewModule(),
		smtp.NewModule(),
	)
}
