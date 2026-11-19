package events

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"syscall"
)

var errNonPublicAddress = errors.New("events: endpoint address is not public")

// nonPublicPrefixes are the ranges an endpoint may never reach: loopback,
// private, shared (CGNAT, the tailnet), link-local (cloud metadata),
// unspecified, multicast, documentation, benchmarking, reserved, and the IPv6
// prefixes that embed an IPv4 address (NAT64, 6to4, Teredo). The stdlib
// helpers miss several of them.
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

// isPublicAddress reports whether an endpoint may reach addr. An IPv4 address
// written as IPv6 (::ffff:127.0.0.1) is checked as IPv4.
func isPublicAddress(addr netip.Addr) bool {
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

// isLocalHostname reports the names that resolve to the host itself.
func isLocalHostname(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

// refuseNonPublicDial is a net.Dialer Control hook: it sees the address each
// connection actually dials, after DNS, so a name that resolves to a private
// address, or changes to one after the endpoint was saved, is refused too.
func refuseNonPublicDial(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", errNonPublicAddress, address)
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || !isPublicAddress(addr) {
		return fmt.Errorf("%w: %s", errNonPublicAddress, host)
	}
	return nil
}
