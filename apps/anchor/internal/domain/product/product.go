package product

import (
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

const DefaultAPIKeyPrefix = "anchor"

type Config struct {
	APIKeys APIKeysConfig
}

type APIKeysConfig struct {
	Prefix string
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
	return Config{APIKeys: APIKeysConfig{Prefix: DefaultAPIKeyPrefix}}
}

func (c Config) WithDefaults() Config {
	if c.APIKeys.Prefix == "" {
		c.APIKeys.Prefix = DefaultAPIKeyPrefix
	}

	return c
}
