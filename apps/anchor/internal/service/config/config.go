package config

type CoreConfig struct {
	Auth        AuthConfig        `yaml:"auth"`
	Encryption  EncryptionConfig  `yaml:"encryption"`
	Integration IntegrationConfig `yaml:"integration"`
	Webhooks    WebhooksConfig    `yaml:"webhooks"`
	Environment string            `yaml:"environment"`
}

type AuthConfig struct {
	RefreshTokenLifetime int64  `yaml:"refresh_token_lifetime"`
	AccessTokenLifetime  int64  `yaml:"access_token_lifetime"`
	AdminJWTSecret       string `yaml:"admin_jwt_secret"`
}

type EncryptionConfig struct {
	GlobalKey        string `yaml:"global_key"`
	GlobalKeyVersion string `yaml:"global_key_version"`
}

type IntegrationConfig struct {
	ReconcileScheduleInterval string `yaml:"reconcile_schedule_interval"`
}

// WebhooksConfig tunes outbound webhook delivery.
type WebhooksConfig struct {
	// AllowInsecureTargets relaxes the outbound target policy: plain http URLs
	// are accepted and the private/loopback address block is not applied.
	//
	// It exists so integration tests can point an endpoint at a container on
	// localhost. It MUST stay false everywhere else: with it on, a product
	// administrator can aim a webhook at any service reachable from Anchor.
	AllowInsecureTargets bool `yaml:"allow_insecure_targets"`
}

func (a AuthConfig) GetAdminJWTSecretAsBytes() []byte {
	return []byte(a.AdminJWTSecret)
}

func (c *CoreConfig) IsDevelopment() bool {
	return c.Environment == "development"
}

func (c *CoreConfig) IsProduction() bool {
	return c.Environment == "production"
}
