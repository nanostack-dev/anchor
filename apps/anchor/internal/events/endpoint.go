package events

import (
	"net/netip"
	"net/url"
	"time"
)

type Endpoint struct {
	ProductID               string
	PlatformTenantID        string
	URL                     string
	SigningSecretEncrypted  string
	SigningSecretClear      string
	SigningSecretObfuscated string
	Events                  []string
	DeliveryStatus          string
	ConsecutiveFailedCalls  int32
	// FailingSince is the first failed call of the current streak, and
	// LastFailedAt its latest one. Both are zero while the endpoint is not
	// failing.
	FailingSince time.Time
	LastFailedAt time.Time
}

type UpsertEndpointInput struct {
	TenantID  string `validate:"required,notblank"`
	ProductID string `validate:"required,notblank"`
	URL       string `validate:"required,notblank"`
	Events    []string
}

// EndpointURLRules are the environment's rules for an endpoint URL.
type EndpointURLRules struct {
	// RequireHTTPS refuses http URLs (production).
	RequireHTTPS bool
	// AllowPrivate accepts loopback, private and other non-public addresses
	// and localhost. Only the test suites and a local run set it, through
	// core.events.allow_private_endpoints.
	AllowPrivate bool
}

// validateEndpointURL gives early feedback on a URL that can never be
// delivered to. The delivery dial checks the resolved address again, which
// covers names, other IP spellings and DNS changes.
func validateEndpointURL(raw string, rules EndpointURLRules) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return invalidEndpointURLError()
	}
	if rules.RequireHTTPS && parsed.Scheme != "https" {
		return insecureEndpointURLError()
	}
	if rules.AllowPrivate {
		return nil
	}
	host := parsed.Hostname()
	if isLocalHostname(host) {
		return nonPublicEndpointURLError()
	}
	if addr, parseErr := netip.ParseAddr(host); parseErr == nil && !isPublicAddress(addr) {
		return nonPublicEndpointURLError()
	}
	return nil
}
