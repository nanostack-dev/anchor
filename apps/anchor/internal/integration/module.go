package integration

import (
	"github.com/nanostack-dev/nanostack-framework/core/nano"

	"anchor/internal/integration/provider"
	"anchor/internal/integration/provider/clerk"
	"anchor/internal/integration/provider/smtp"
	"anchor/internal/security/encryption"
)

// Module wires the integration subsystem: encryption, provider registry,
// and all registered integration providers.
var Module = nano.Module("integration", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Providers(encryption.NewService),
	provider.Module,
	clerk.Module,
	smtp.Module,
)
