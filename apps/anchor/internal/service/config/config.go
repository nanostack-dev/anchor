package config

type CoreConfig struct {
	Auth        AuthConfig        `yaml:"auth"`
	Encryption  EncryptionConfig  `yaml:"encryption"`
	Integration IntegrationConfig `yaml:"integration"`
	Workflow    WorkflowConfig    `yaml:"workflow"`
	Environment string            `yaml:"environment"`
}

// WorkflowConfig tunes workflow steps that call out. AllowPrivateTargets lets
// a step reach plain HTTP and private, loopback and link-local addresses; it
// exists for the test suites, whose stub backends listen on loopback, and
// stays off in every deployed environment.
type WorkflowConfig struct {
	AllowPrivateTargets bool `yaml:"allow_private_targets"`
}

type AuthConfig struct {
	RefreshTokenLifetime int64  `yaml:"refresh_token_lifetime"`
	SessionMaxLifetime   int64  `yaml:"session_max_lifetime"`
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
