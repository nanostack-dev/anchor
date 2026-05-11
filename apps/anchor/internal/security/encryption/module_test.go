//nolint:testpackage // tests require access to unexported NewService constructor directly
package encryption

import (
	"encoding/base64"
	"testing"

	serviceconfig "anchor/internal/service/config"

	"github.com/stretchr/testify/require"
)

func TestNewServiceRequiresValidGlobalKey(t *testing.T) {
	t.Parallel()

	_, err := NewService(&serviceconfig.CoreConfig{})
	require.Error(t, err)
}

func TestNewServiceExplainsLegacyRawKeyFailure(t *testing.T) {
	t.Parallel()

	_, err := NewService(&serviceconfig.CoreConfig{
		Encryption: serviceconfig.EncryptionConfig{
			GlobalKey:        "0123456789abcdef0123456789abcdef",
			GlobalKeyVersion: "v1",
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid APP_ENCRYPTION_KEY")
	require.ErrorContains(t, err, "base64-encoded 32-byte key")
	require.ErrorContains(t, err, "legacy raw 32-character secret")
}

func TestServiceBuildsUsableCipher(t *testing.T) {
	t.Parallel()

	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 1)
	}

	service, err := NewService(&serviceconfig.CoreConfig{
		Encryption: serviceconfig.EncryptionConfig{
			GlobalKey:        base64.StdEncoding.EncodeToString(rawKey),
			GlobalKeyVersion: "v7",
		},
	})
	require.NoError(t, err)

	cipher, err := service.NewCipher("clerk-api-key")
	require.NoError(t, err)
	require.Equal(t, "v7", cipher.CurrentVersion())

	encrypted, err := cipher.EncryptString("secret")
	require.NoError(t, err)

	plain, err := cipher.DecryptString(encrypted)
	require.NoError(t, err)
	require.Equal(t, "secret", plain)
}
