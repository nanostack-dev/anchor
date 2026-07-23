package webhook_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/webhook"
)

func TestIsBlockedIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
		want bool
	}{
		// Allowed: ordinary public destinations.
		{name: "public IPv4", addr: "93.184.216.34"},
		{name: "public IPv4 cloudflare", addr: "1.1.1.1"},
		{name: "public IPv6", addr: "2606:4700:4700::1111"},

		// Loopback in every notation.
		{name: "IPv4 loopback", addr: "127.0.0.1", want: true},
		{name: "IPv4 loopback elsewhere in 127/8", addr: "127.9.9.9", want: true},
		{name: "IPv6 loopback", addr: "::1", want: true},
		{name: "IPv4-mapped IPv6 loopback", addr: "::ffff:127.0.0.1", want: true},

		// RFC1918 private space, including the mapped forms.
		{name: "10/8", addr: "10.0.0.5", want: true},
		{name: "172.16/12", addr: "172.20.1.1", want: true},
		{name: "192.168/16", addr: "192.168.1.10", want: true},
		{name: "IPv4-mapped IPv6 private", addr: "::ffff:192.168.1.10", want: true},
		{name: "IPv6 unique local", addr: "fd00::1", want: true},

		// Link-local, which is where the cloud metadata endpoint lives.
		{name: "link-local", addr: "169.254.1.1", want: true},
		{name: "cloud metadata endpoint", addr: "169.254.169.254", want: true},
		{name: "IPv4-mapped cloud metadata endpoint", addr: "::ffff:169.254.169.254", want: true},
		{name: "IPv6 link-local", addr: "fe80::1", want: true},

		// Multicast and unspecified.
		{name: "IPv4 multicast", addr: "224.0.0.1", want: true},
		{name: "IPv6 multicast", addr: "ff02::1", want: true},
		{name: "IPv4 unspecified", addr: "0.0.0.0", want: true},
		{name: "IPv6 unspecified", addr: "::", want: true},

		// Reserved ranges Go's own predicates do not cover.
		{name: "this network 0/8", addr: "0.1.2.3", want: true},
		{name: "carrier-grade NAT", addr: "100.64.1.1", want: true},
		{name: "IETF protocol assignments", addr: "192.0.0.1", want: true},
		{name: "benchmarking range", addr: "198.19.0.1", want: true},
		{name: "reserved 240/4", addr: "240.0.0.1", want: true},
		{name: "NAT64 prefix embedding IPv4", addr: "64:ff9b::7f00:1", want: true},
		{name: "documentation IPv6", addr: "2001:db8::1", want: true},

		// A public address just outside a blocked range must stay allowed.
		{name: "just below CGNAT", addr: "100.63.255.255"},
		{name: "just above CGNAT", addr: "100.128.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			addr, err := netip.ParseAddr(tt.addr)
			require.NoError(t, err)
			assert.Equal(t, tt.want, webhook.IsBlockedIP(addr))
		})
	}
}

func TestIsBlockedIPRejectsAnInvalidAddress(t *testing.T) {
	t.Parallel()

	// If we cannot tell what we are about to dial, we do not dial it.
	assert.True(t, webhook.IsBlockedIP(netip.Addr{}))
}

func TestIsBlockedAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{name: "public ip:port", address: "93.184.216.34:443"},
		{name: "loopback ip:port", address: "127.0.0.1:8080", want: true},
		{name: "bracketed IPv6 loopback", address: "[::1]:8080", want: true},
		{name: "unparseable address is blocked", address: "not-an-address", want: true},
		{name: "hostname instead of ip is blocked", address: "example.com:443", want: true},
		{name: "missing port is blocked", address: "93.184.216.34", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, webhook.IsBlockedAddress(tt.address))
		})
	}
}

func TestValidateTargetURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		raw           string
		allowInsecure bool
		wantErr       bool
	}{
		{name: "https is always allowed", raw: "https://example.com/hooks"},
		{name: "http is refused by default", raw: "http://example.com/hooks", wantErr: true},
		{name: "http is allowed when explicitly relaxed", raw: "http://example.com/hooks", allowInsecure: true},
		{name: "ftp is never allowed", raw: "ftp://example.com/hooks", wantErr: true},
		{name: "file is never allowed", raw: "file:///etc/passwd", wantErr: true},
		{name: "a relative url has no host", raw: "/hooks", wantErr: true},
		{name: "empty", raw: "", wantErr: true},
		{name: "surrounding whitespace is tolerated", raw: "  https://example.com/hooks  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := webhook.ValidateTargetURL(tt.raw, tt.allowInsecure)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
