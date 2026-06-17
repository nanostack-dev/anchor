package httpserver

type ServerConfig struct {
	Port int `yaml:"port"`
	// AllowedOrigin is a comma-separated CORS origin list. Entries may be exact
	// (https://app.tryanchor.dev) or wildcard (https://*.tryanchor.dev); go-chi
	// matches wildcards and reflects the matched origin. Prod lists exact
	// origins; dev/preview add a wildcard. No domain literals in source.
	AllowedOrigin string `yaml:"allowed_origin"`
}
