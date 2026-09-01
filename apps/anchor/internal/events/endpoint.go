package events

import (
	"net"
	"net/url"
	"strings"
)

type Endpoint struct {
	ProductID               string
	PlatformTenantID        string
	URL                     string
	SigningSecretEncrypted  string
	SigningSecretClear      string
	SigningSecretObfuscated string
}

type UpsertEndpointInput struct {
	TenantID      string `validate:"required,notblank"`
	ProductID     string `validate:"required,notblank"`
	URL           string `validate:"required,notblank"`
	SigningSecret string
}

func validateEndpointURL(raw string, production bool) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return invalidEndpointURLError()
	}
	if production && parsed.Scheme != "https" {
		return insecureEndpointURLError()
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip != nil && ip.IsLinkLocalUnicast() {
		return invalidEndpointURLError()
	}
	if strings.EqualFold(host, "169.254.169.254") {
		return invalidEndpointURLError()
	}
	return nil
}
