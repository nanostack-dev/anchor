// Package netguard validates outbound hostnames and IP addresses against
// non-public network ranges (loopback, private, link-local / cloud metadata,
// CGNAT, multicast, reserved, and IPv4-embedding IPv6 prefixes) both at
// configuration time and at TCP dial time via net.Dialer.Control.
package netguard

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"syscall"
)

// ErrNonPublicAddress is returned when an outbound host or resolved IP address
// falls outside the public internet.
var ErrNonPublicAddress = errors.New("address is not public")

// nonPublicPrefixes are the ranges an outbound integration or webhook may never
// reach: loopback, private, shared (CGNAT, tailnet), link-local (cloud
// metadata), unspecified, multicast, documentation, benchmarking, reserved, and
// the IPv6 prefixes that embed an IPv4 address (NAT64, 6to4, Teredo). The
// stdlib helpers miss several of them.
func nonPublicPrefixes() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("192.88.99.0/24"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("224.0.0.0/4"),
		netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("::/128"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("64:ff9b::/96"),
		netip.MustParsePrefix("64:ff9b:1::/48"),
		netip.MustParsePrefix("100::/64"),
		netip.MustParsePrefix("2001::/23"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("2002::/16"),
		netip.MustParsePrefix("fc00::/7"),
		netip.MustParsePrefix("fe80::/10"),
		netip.MustParsePrefix("fec0::/10"),
		netip.MustParsePrefix("ff00::/8"),
	}
}

// IsPublicAddress reports whether addr is a globally routable public IP
// address. An IPv4 address written as IPv6 (::ffff:127.0.0.1) is checked as
// IPv4, and any IPv6 zone identifier is stripped before checking.
func IsPublicAddress(addr netip.Addr) bool {
	addr = addr.Unmap().WithZone("")
	if !addr.IsValid() {
		return false
	}
	for _, prefix := range nonPublicPrefixes() {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

// IsLocalHostname reports names that resolve to the host itself (localhost,
// *.localhost, localhost.localdomain).
func IsLocalHostname(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	return host == "localhost" ||
		host == "localhost.localdomain" ||
		strings.HasSuffix(host, ".localhost")
}

// ValidatePublicHost checks a configured hostname or IP literal before saving.
// Rejects local hostnames and any non-public IP literal (including bracketed
// IPv6 literals and zoned IPv6 addresses). Hostnames that resolve via DNS are
// re-checked post-resolution by RefuseNonPublicDial.
func ValidatePublicHost(host string) error {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return fmt.Errorf("%w: empty host", ErrNonPublicAddress)
	}
	unbracketed := strings.TrimPrefix(strings.TrimSuffix(trimmed, "]"), "[")
	if IsLocalHostname(unbracketed) {
		return fmt.Errorf("%w: %s", ErrNonPublicAddress, trimmed)
	}
	if addr, err := netip.ParseAddr(unbracketed); err == nil {
		if !IsPublicAddress(addr) {
			return fmt.Errorf("%w: %s", ErrNonPublicAddress, trimmed)
		}
	}
	return nil
}

// RefuseNonPublicDial is a net.Dialer Control hook: it sees the concrete IP
// address each connection dials after DNS resolution, so a hostname that
// resolves to a private/loopback/metadata IP (or rebinds after save) is refused.
func RefuseNonPublicDial(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrNonPublicAddress, address)
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || !IsPublicAddress(addr) {
		return fmt.Errorf("%w: %s", ErrNonPublicAddress, host)
	}
	return nil
}
