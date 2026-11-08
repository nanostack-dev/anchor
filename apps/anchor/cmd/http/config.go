package httpserver

import "github.com/nanostack-dev/nanostack-framework/http/nanoserver"

// Config is the server section: the listener settings of nanoserver (port,
// timeouts, drain) and the CORS origins of the router.
type Config struct {
	nanoserver.Config

	// AllowedOrigin is a comma-separated CORS origin list. Entries may be exact
	// (https://app.tryanchor.dev) or wildcard (https://*.tryanchor.dev); go-chi
	// matches wildcards and reflects the matched origin. Prod lists exact
	// origins; dev/preview add a wildcard. No domain literals in source.
	AllowedOrigin string `config:"allowed_origin" env:"ALLOWED_ORIGIN"`
}

// ListenerConfig is the nanoserver part of the server section.
func ListenerConfig(c Config) nanoserver.Config { return c.Config }
