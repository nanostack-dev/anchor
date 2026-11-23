package events

import (
	"fmt"
	"net/netip"
	"syscall"

	"anchor/internal/netguard"
)

var errNonPublicAddress = netguard.ErrNonPublicAddress

func isPublicAddress(addr netip.Addr) bool {
	return netguard.IsPublicAddress(addr)
}

func isLocalHostname(host string) bool {
	return netguard.IsLocalHostname(host)
}

func refuseNonPublicDial(network, address string, rawConn syscall.RawConn) error {
	if err := netguard.RefuseNonPublicDial(network, address, rawConn); err != nil {
		return fmt.Errorf("events: %w", err)
	}
	return nil
}
