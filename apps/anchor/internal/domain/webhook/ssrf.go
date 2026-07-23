package webhook

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
)

// ErrBlockedTarget is returned when a webhook target resolves to an address
// range Anchor refuses to dial.
var ErrBlockedTarget = errors.New("webhook target address is not allowed")

// blockedPrefixes are ranges Go's own address predicates do not classify but
// that are still unsafe to dial from inside our network. Parsing them per call
// keeps the policy immutable; the cost is invisible next to the DNS, TCP and
// TLS work of the dial it guards.
func blockedPrefixes() []netip.Prefix {
	return []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),       // "this network"
		netip.MustParsePrefix("100.64.0.0/10"),   // carrier-grade NAT
		netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
		netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1
		netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
		netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2
		netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3
		netip.MustParsePrefix("240.0.0.0/4"),     // reserved
		netip.MustParsePrefix("64:ff9b::/96"),    // NAT64, embeds arbitrary IPv4
		netip.MustParsePrefix("::/128"),          // unspecified
		netip.MustParsePrefix("2001:db8::/32"),   // documentation
	}
}

// IsBlockedIP reports whether an address is off limits for outbound webhook
// delivery: loopback, RFC1918 private space, link-local (which covers the
// 169.254.169.254 cloud metadata endpoint), multicast, unspecified, and the
// reserved ranges above.
//
// IPv4-mapped IPv6 forms (::ffff:127.0.0.1) are unmapped first, so the same
// address cannot slip through by changing notation.
//
// This check is applied post-resolution, from net.Dialer.Control, with the
// literal ip:port about to be dialed. Validating the hostname at registration
// time is UX, not security: DNS rebinding defeats it, because the name can
// resolve differently at send time.
func IsBlockedIP(addr netip.Addr) bool {
	if !addr.IsValid() {
		return true
	}
	if addr.Is4In6() {
		addr = addr.Unmap()
	}

	if addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() {
		return true
	}

	for _, prefix := range blockedPrefixes() {
		if prefix.Addr().Is4() != addr.Is4() {
			continue
		}
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

// IsBlockedAddress parses a literal "ip:port" as handed to net.Dialer.Control
// and applies IsBlockedIP. An unparseable address is blocked: if we cannot tell
// what we are about to dial, we do not dial it.
func IsBlockedAddress(address string) bool {
	addrPort, err := netip.ParseAddrPort(address)
	if err != nil {
		return true
	}

	return IsBlockedIP(addrPort.Addr())
}

// ValidateTargetURL applies the registration-time checks on an endpoint URL:
// a parseable absolute URL, a host, and HTTPS unless insecure targets are
// explicitly allowed (development and tests only).
func ValidateTargetURL(raw string, allowInsecure bool) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("webhook url is not a valid URL: %w", err)
	}
	if parsed.Host == "" {
		return errors.New("webhook url must include a host")
	}

	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if allowInsecure {
			return nil
		}

		return errors.New("webhook url must use https")
	default:
		return fmt.Errorf("webhook url scheme %q is not supported", parsed.Scheme)
	}
}
