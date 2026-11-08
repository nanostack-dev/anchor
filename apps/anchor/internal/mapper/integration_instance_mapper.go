package mapper

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/domain/integration"
	"anchor/internal/security/encryption"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
)

// webhookSecretCipherContext isolates the Svix signing secrets from the other secret classes.
const webhookSecretCipherContext = "integration-webhook-secret"

// IntegrationInstanceMapper stores the webhook signing secret encrypted. A row written before encryption
// holds plaintext, which ToDomain still reads; the next write of that instance encrypts it.
type IntegrationInstanceMapper struct {
	cipher *secrets.VersionedCipher
	logger *slog.Logger
}

func NewIntegrationInstanceMapper(
	encryptionService *encryption.Service, logger *slog.Logger,
) (*IntegrationInstanceMapper, error) {
	cipher, err := encryptionService.NewCipher(webhookSecretCipherContext)
	if err != nil {
		return nil, fmt.Errorf("webhook secret cipher: %w", err)
	}
	return &IntegrationInstanceMapper{cipher: cipher, logger: logger}, nil
}

func (m *IntegrationInstanceMapper) ToDomain(entity model.IntegrationInstances) integration.Instance {
	webhookSecret := functional.FromPtr(entity.WebhookSecret).ToPtr()
	if webhookSecret != nil && secrets.IsVersionedEncryptedSecret(*webhookSecret) {
		decrypted, err := m.cipher.DecryptString(*webhookSecret)
		if err != nil {
			// Without the secret every signature check fails closed.
			m.logger.Error("decrypt webhook secret", log.Err(err), slog.String("instance_id", entity.ID))
			decrypted = ""
		}
		webhookSecret = &decrypted
	}
	lastError := functional.FromPtr(entity.LastError).ToPtr()

	return integration.Instance{
		ID:               entity.ID,
		PlatformTenantID: entity.PlatformTenantID,
		ProductID:        entity.ProductID,
		ProviderType:     integration.ProviderType(entity.ProviderType),
		WebhookSecret:    webhookSecret,
		ConfigJSON:       json.RawMessage(entity.ConfigJSON),
		ConfigVersion:    entity.ConfigVersion,
		IsEnabled:        entity.IsEnabled,
		LastError:        lastError,
		Status:           integration.Status(entity.Status),
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
	}
}

func (m *IntegrationInstanceMapper) ToEntity(domain integration.Instance) model.IntegrationInstances {
	webhookSecret := functional.FromPtr(domain.WebhookSecret).ToPtr()
	if webhookSecret != nil && *webhookSecret != "" && !secrets.IsVersionedEncryptedSecret(*webhookSecret) {
		encrypted, err := m.cipher.EncryptString(*webhookSecret)
		if err != nil {
			// Never store the plaintext when encryption fails; the instance then needs its secret again.
			m.logger.Error("encrypt webhook secret", log.Err(err), slog.String("instance_id", domain.ID))
			encrypted = ""
		}
		webhookSecret = &encrypted
	}
	lastError := functional.FromPtr(domain.LastError).ToPtr()

	configStr := "{}"
	if domain.ConfigJSON != nil {
		configStr = string(domain.ConfigJSON)
	}

	return model.IntegrationInstances{
		ID:               domain.ID,
		PlatformTenantID: domain.PlatformTenantID,
		ProductID:        domain.ProductID,
		ProviderType:     string(domain.ProviderType),
		WebhookSecret:    webhookSecret,
		ConfigJSON:       configStr,
		ConfigVersion:    domain.ConfigVersion,
		IsEnabled:        domain.IsEnabled,
		LastError:        lastError,
		Status:           string(domain.Status),
		CreatedAt:        domain.CreatedAt,
		UpdatedAt:        domain.UpdatedAt,
	}
}
