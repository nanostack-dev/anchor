package encryption

import (
	"errors"
	"fmt"
	"strings"

	"anchor/internal/service/config"

	"github.com/nanostack-dev/shared/toolkit"
	"go.uber.org/fx"
)

const defaultKeyVersion = "v1"

type Service struct {
	globalKey        string
	globalKeyVersion string
}

func NewService(coreCfg *config.CoreConfig) (*Service, error) {
	version := coreCfg.Encryption.GlobalKeyVersion
	if version == "" {
		version = defaultKeyVersion
	}

	if _, err := toolkit.NewVersionedSecretCipherWithSingleKey(
		version,
		"validation",
		coreCfg.Encryption.GlobalKey,
	); err != nil {
		return nil, fmt.Errorf(
			"invalid APP_ENCRYPTION_KEY for version %q: %w. APP_ENCRYPTION_KEY must be a base64-encoded 32-byte key. A legacy raw 32-character secret will now fail validation because it is decoded as base64 first instead of being hashed to 32 bytes",
			version,
			normalizeValidationError(err),
		)
	}

	return &Service{
		globalKey:        coreCfg.Encryption.GlobalKey,
		globalKeyVersion: version,
	}, nil
}

func normalizeValidationError(err error) error {
	if err == nil {
		return nil
	}

	message := err.Error()
	if strings.Contains(message, "illegal base64 data") {
		return errors.New("value is not valid base64")
	}

	return err
}

// NewCipher returns a context-scoped cipher derived from the global app key.
// Callers should pass a stable domain string (for example, "clerk-api-key") so
// each secret class is cryptographically isolated by context.
func (s *Service) NewCipher(context string) (*toolkit.VersionedSecretCipher, error) {
	return toolkit.NewVersionedSecretCipherWithSingleKey(
		s.globalKeyVersion,
		context,
		s.globalKey,
	)
}

func NewModule() fx.Option {
	return fx.Module(
		"encryption",
		fx.Provide(
			NewService,
		),
	)
}
