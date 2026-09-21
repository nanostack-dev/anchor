package product

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

const DefaultOrganizationAPIKeyRootPrefix = "anchor"

type Config struct {
	OrganizationAPIKeys OrganizationAPIKeysConfig
	Events              *EventsConfig
}

type OrganizationAPIKeysConfig struct {
	Prefix string
}

type EventsConfig struct {
	EndpointURL             string
	SigningSecret           string
	SigningSecretObfuscated string
}

type Product struct {
	ID               string
	PlatformTenantID string
	Name             string
	Description      string
	Config           Config
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// GenerateID sets the product's ID to a new prefixed KSUID.
func (p *Product) GenerateID() {
	p.ID = ids.MustNew("prd")
}

func DefaultConfig() Config {
	return Config{
		OrganizationAPIKeys: OrganizationAPIKeysConfig{Prefix: DefaultOrganizationAPIKeyRootPrefix},
	}
}

func (c Config) WithDefaults() Config {
	if c.OrganizationAPIKeys.Prefix == "" {
		c.OrganizationAPIKeys.Prefix = DefaultOrganizationAPIKeyRootPrefix
	}

	return c
}
