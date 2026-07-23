package service

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const licenseSigningKeyStartupLockKey = "license_signing_keys:startup-ensure"

type licenseSigningKeyStartupParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Lock      *pglock.Client
	Signer    LicenseSigningService
	Logger    zerolog.Logger
}

// RegisterLicenseSigningKeyStartupEnsure guarantees at startup — under an
// advisory lock so concurrent replicas don't race — that one ACTIVE license
// signing key exists, generating an Ed25519 keypair when none does. Failures
// are logged but do not block startup; token issuance surfaces a typed error
// until a key exists.
func RegisterLicenseSigningKeyStartupEnsure(p licenseSigningKeyStartupParams) {
	logger := p.Logger.With().Str("component", "license_signing_key_startup").Logger()

	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			acquired, err := p.Lock.TryWithLock(
				ctx,
				licenseSigningKeyStartupLockKey,
				func(lockCtx context.Context, _ *sql.Tx) error {
					_, ensureErr := p.Signer.EnsureActiveSigningKey(lockCtx)
					return ensureErr
				},
			)
			if err != nil {
				logger.Error().Err(err).Msg("startup license signing key ensure failed")
				return nil
			}
			if !acquired {
				logger.Debug().Msg("startup license signing key ensure skipped: lock not acquired")
			}

			return nil
		},
	})
}
