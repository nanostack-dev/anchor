package events

import (
	"context"
	"slices"

	"anchor/internal/security/encryption"

	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"
)

const cipherContext = "product-event-endpoint"

type DeliveryTarget struct {
	URL    string
	Secret string
	Events []string
}

func (t DeliveryTarget) Allows(eventType Type) bool {
	if t.Events == nil {
		return true
	}
	return slices.Contains(t.Events, string(eventType))
}

type EndpointService interface {
	Upsert(ctx context.Context, input UpsertEndpointInput) (Endpoint, error)
	Get(ctx context.Context, tenantID, productID string) (Endpoint, bool, error)
	Clear(ctx context.Context, tenantID, productID string) error
	DeliveryTarget(ctx context.Context, productID string) (DeliveryTarget, bool, error)
	Catalog() Catalog
}

type endpointService struct {
	repo       EndpointRepository
	catalog    Catalog
	cipher     *secrets.VersionedCipher
	production bool
	logger     zerolog.Logger
}

func NewEndpointService(
	repo EndpointRepository,
	catalog Catalog,
	enc *encryption.Service,
	production bool,
	logger zerolog.Logger,
) (EndpointService, error) {
	cipher, err := enc.NewCipher(cipherContext)
	if err != nil {
		return nil, err
	}
	return &endpointService{
		repo:       repo,
		catalog:    catalog,
		cipher:     cipher,
		production: production,
		logger:     logger.With().Str("component", "event_endpoint_service").Logger(),
	}, nil
}

func (s *endpointService) Catalog() Catalog {
	return s.catalog
}

func (s *endpointService) Upsert(ctx context.Context, input UpsertEndpointInput) (Endpoint, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return Endpoint{}, err
	}
	if err := validateEndpointURL(input.URL, s.production); err != nil {
		return Endpoint{}, err
	}

	secret, generated, err := s.resolveSigningSecret(ctx, input.TenantID, input.ProductID)
	if err != nil {
		return Endpoint{}, err
	}

	encrypted, err := s.cipher.EncryptString(secret)
	if err != nil {
		return Endpoint{}, err
	}

	eventsList := input.Events
	if eventsList == nil && s.catalog != nil {
		eventsList = s.catalog.AllEventTypesStrings()
	}

	endpoint := Endpoint{
		ProductID:               input.ProductID,
		PlatformTenantID:        input.TenantID,
		URL:                     input.URL,
		SigningSecretEncrypted:  encrypted,
		SigningSecretObfuscated: secrets.Obfuscate(secret),
		Events:                  eventsList,
	}
	if generated {
		endpoint.SigningSecretClear = secret
	}
	if upsertErr := s.repo.Upsert(ctx, endpoint); upsertErr != nil {
		return Endpoint{}, upsertErr
	}
	s.logger.Info().Str("product_id", input.ProductID).Msg("event endpoint upserted")
	return endpoint, nil
}

func (s *endpointService) Get(ctx context.Context, tenantID, productID string) (Endpoint, bool, error) {
	found, err := s.repo.FindByProductIDInternal(ctx, productID)
	if err != nil {
		return Endpoint{}, false, err
	}
	if found.IsAbsent() {
		return Endpoint{}, false, nil
	}
	endpoint := found.Value()
	if endpoint.PlatformTenantID != tenantID {
		return Endpoint{}, false, nil
	}
	plaintext, err := s.cipher.DecryptString(endpoint.SigningSecretEncrypted)
	if err != nil {
		return Endpoint{}, false, err
	}
	endpoint.SigningSecretObfuscated = secrets.Obfuscate(plaintext)
	return endpoint, true, nil
}

func (s *endpointService) Clear(ctx context.Context, tenantID, productID string) error {
	return s.repo.Delete(ctx, tenantID, productID)
}

func (s *endpointService) resolveSigningSecret(
	ctx context.Context, tenantID, productID string,
) (string, bool, error) {
	found, err := s.repo.FindByProductIDInternal(ctx, productID)
	if err != nil {
		return "", false, err
	}
	if found.IsPresent() && found.Value().PlatformTenantID == tenantID {
		plaintext, decryptErr := s.cipher.DecryptString(found.Value().SigningSecretEncrypted)
		if decryptErr != nil {
			return "", false, decryptErr
		}
		return plaintext, false, nil
	}
	generated, genErr := NewSigningSecret()
	if genErr != nil {
		return "", false, genErr
	}
	return generated, true, nil
}

func (s *endpointService) DeliveryTarget(
	ctx context.Context, productID string,
) (DeliveryTarget, bool, error) {
	found, err := s.repo.FindByProductIDInternal(ctx, productID)
	if err != nil {
		return DeliveryTarget{}, false, err
	}
	if found.IsAbsent() {
		return DeliveryTarget{}, false, nil
	}
	endpoint := found.Value()
	secret, err := s.cipher.DecryptString(endpoint.SigningSecretEncrypted)
	if err != nil {
		return DeliveryTarget{}, false, err
	}
	return DeliveryTarget{URL: endpoint.URL, Secret: secret, Events: endpoint.Events}, true, nil
}
