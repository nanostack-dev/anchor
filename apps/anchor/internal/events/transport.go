package events

import (
	"net"
	"net/http"
	"time"

	"anchor/internal/telemetry"
)

const (
	deliveryDialTimeout         = 10 * time.Second
	deliveryKeepAlive           = 30 * time.Second
	deliveryTLSHandshakeTimeout = 10 * time.Second
	deliveryIdleConnTimeout     = 90 * time.Second
	deliveryMaxIdleConns        = 100
)

// newDeliveryClient is the worker's HTTP client. Unless allowPrivate, its
// dialer refuses every non-public address after DNS. It uses no proxy, so the
// check sees the endpoint's address rather than a proxy's, and it never
// follows a redirect.
func newDeliveryClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{Timeout: deliveryDialTimeout, KeepAlive: deliveryKeepAlive}
	if !allowPrivate {
		dialer.Control = refuseNonPublicDial
	}
	transport := &http.Transport{
		Proxy:               nil,
		DialContext:         dialer.DialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        deliveryMaxIdleConns,
		IdleConnTimeout:     deliveryIdleConnTimeout,
		TLSHandshakeTimeout: deliveryTLSHandshakeTimeout,
	}
	return &http.Client{
		Transport: telemetry.ExternalTransport(transport),
		Timeout:   deliveryTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
