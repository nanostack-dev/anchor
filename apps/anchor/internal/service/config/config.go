package config

type CoreConfig struct {
	Auth        AuthConfig        `yaml:"auth"`
	Encryption  EncryptionConfig  `yaml:"encryption"`
	Integration IntegrationConfig `yaml:"integration"`
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

func (a AuthConfig) GetAdminJWTSecretAsBytes() []byte {
	return []byte(a.AdminJWTSecret)
}

func (c *CoreConfig) IsDevelopment() bool {
	return c.Environment == "development"
}

func (c *CoreConfig) IsProduction() bool {
	return c.Environment == "production"
}
