package smtp

import (
	"log/slog"

	"anchor/internal/integration/provider"
	"anchor/internal/security/encryption"
	serviceconfig "anchor/internal/service/config"

	"github.com/nanostack-dev/nanostack-framework/core/nano"
)

// Module registers the SMTP provider into the integration provider registry.
var Module = nano.Module("smtp_provider", //nolint:gochecknoglobals // A nano module is a package-level value.
	nano.Collect[provider.Provider](ProvideProvider),
)

// ProvideProvider wires the SMTP provider with the deployment's outbound
// private-host policy from core.integration.allow_private_hosts.
func ProvideProvider(
	encryptionService *encryption.Service,
	core *serviceconfig.CoreConfig,
	logger *slog.Logger,
) *Provider {
	allowPrivate := core != nil && core.Integration.AllowPrivateHosts
	return NewProviderWithOptions(encryptionService, allowPrivate, logger)
}
