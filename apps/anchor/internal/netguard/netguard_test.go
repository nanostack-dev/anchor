package netguard_test

import (
	"net/netip"
	"testing"

	"anchor/internal/netguard"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPublicAddress(t *testing.T) {
	t.Parallel()

	nonPublic := []string{
		"0.0.0.0",
		"10.0.0.1",
		"100.64.0.1",
		"127.0.0.1",
		"169.254.169.254",
		"172.16.0.1",
		"192.168.1.1",
		"224.0.0.1",
		"::",
		"::1",
		"::ffff:127.0.0.1",
		"::ffff:169.254.169.254",
		"64:ff9b::127.0.0.1",
		"fc00::1",
		"fe80::1",
		"ff02::1",
	}
	for _, raw := range nonPublic {
		addr := netip.MustParseAddr(raw)
		assert.False(t, netguard.IsPublicAddress(addr), "expected %s to be non-public", raw)
	}

	public := []string{
		"1.1.1.1",
		"8.8.8.8",
		"54.240.8.1",
		"2606:4700:4700::1111",
	}
	for _, raw := range public {
		addr := netip.MustParseAddr(raw)
		assert.True(t, netguard.IsPublicAddress(addr), "expected %s to be public", raw)
	}
}

func TestValidatePublicHost(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"",
		"localhost",
		"LOCALHOST.",
		"smtp.localhost",
		"localhost.localdomain",
		"127.0.0.1",
		"169.254.169.254",
		"10.0.0.5",
		"192.168.0.1",
		"[::1]",
		"::ffff:127.0.0.1",
		"fe80::1%eth0",
	}
	for _, host := range blocked {
		err := netguard.ValidatePublicHost(host)
		require.ErrorIs(t, err, netguard.ErrNonPublicAddress, "expected %q to be blocked", host)
	}

	allowed := []string{
		"smtp.sendgrid.net",
		"email-smtp.us-east-1.amazonaws.com",
		"1.1.1.1",
		"2606:4700:4700::1111",
	}
	for _, host := range allowed {
		assert.NoError(t, netguard.ValidatePublicHost(host), "expected %q to be allowed", host)
	}
}

func TestRefuseNonPublicDial(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, netguard.RefuseNonPublicDial("tcp", "127.0.0.1:25", nil), netguard.ErrNonPublicAddress)
	require.ErrorIs(t, netguard.RefuseNonPublicDial("tcp", "169.254.169.254:80", nil), netguard.ErrNonPublicAddress)
	require.ErrorIs(t, netguard.RefuseNonPublicDial("tcp", "[::1]:587", nil), netguard.ErrNonPublicAddress)
	assert.NoError(t, netguard.RefuseNonPublicDial("tcp", "1.1.1.1:587", nil))
}
