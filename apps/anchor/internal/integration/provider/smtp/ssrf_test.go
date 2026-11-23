//nolint:testpackage // Tests unexported dialSMTP alongside exported Provider methods.
package smtp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"testing"

	"anchor/internal/domain/integration"
	"anchor/internal/netguard"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSMTPRejectsNonPublicHostsByDefault(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	strict := NewProvider(nil, logger)

	t.Run("ValidateConfig refuses loopback, private, metadata, and localhost hosts", func(t *testing.T) {
		t.Parallel()
		for _, host := range []string{
			"127.0.0.1",
			"localhost",
			"sub.localhost",
			"169.254.169.254",
			"10.0.0.1",
			"192.168.1.10",
			"::1",
			"::ffff:127.0.0.1",
		} {
			raw, err := json.Marshal(Config{
				Host:        host,
				Port:        587,
				Encryption:  EncryptionStartTLS,
				AuthMethod:  AuthMethodPlain,
				Username:    "user",
				FromAddress: "noreply@example.com",
			})
			require.NoError(t, err)
			err = strict.ValidateConfig(context.Background(), raw)
			require.Error(t, err, "expected ValidateConfig to reject %q", host)
			assert.Contains(t, err.Error(), "public address")
		}
	})

	t.Run("ValidateConfig accepts public SMTP hostnames", func(t *testing.T) {
		t.Parallel()
		raw, err := json.Marshal(Config{
			Host:        "smtp.sendgrid.net",
			Port:        587,
			Encryption:  EncryptionStartTLS,
			AuthMethod:  AuthMethodPlain,
			Username:    "apikey",
			FromAddress: "noreply@example.com",
		})
		require.NoError(t, err)
		assert.NoError(t, strict.ValidateConfig(context.Background(), raw))
	})

	t.Run("dialSMTP blocks loopback after resolution when allowPrivateHosts is false", func(t *testing.T) {
		t.Parallel()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		t.Cleanup(func() { _ = ln.Close() })
		port := ln.Addr().(*net.TCPAddr).Port

		cfg := Config{
			Host:       "127.0.0.1",
			Port:       port,
			Encryption: EncryptionNone,
		}
		_, dialErr := dialSMTP(context.Background(), cfg, false)
		require.ErrorIs(t, dialErr, netguard.ErrNonPublicAddress)

		// VerifyConnection returns a sanitized non-public error without leaking host/port.
		raw, err := json.Marshal(Config{
			Host:        "127.0.0.1",
			Port:        port,
			Encryption:  EncryptionNone,
			AuthMethod:  AuthMethodPlain,
			Username:    "u",
			Password:    "p",
			FromAddress: "noreply@example.com",
		})
		require.NoError(t, err)
		verifyErr := strict.VerifyConnection(context.Background(), &integration.Instance{
			ID:         "iin_test",
			ConfigJSON: raw,
		})
		require.Error(t, verifyErr)
		assert.Equal(t, "smtp: host address is not public", verifyErr.Error())
	})
}
