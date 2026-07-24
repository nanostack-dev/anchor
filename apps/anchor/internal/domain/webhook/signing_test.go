package webhook_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/webhook"
)

// The canonical Standard Webhooks / Svix test vector, published alongside the
// specification. Asserting against it proves interoperability rather than
// self-consistency: any consumer library that verifies this vector verifies
// Anchor's signatures, because the secret decoding, the signing content and the
// entry format are all identical.
const (
	specSecret    = "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
	specMessageID = "msg_p5jXN8AQM9LWM0D4loKWxJek"
	specTimestamp = int64(1614265330)
	specBody      = `{"test": 2432232314}`
	specSignature = "g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE="
	otherSecret   = "whsec_QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVowMTIzNDU2Nzg5"
	otherDelivery = "whd_2iABCDEFGHIJKLMNOPQRSTUVW"
	invalidSecret = "whsec_not valid base64!!"
	unprefixedB64 = "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVowMTIzNDU2Nzg5"
)

func TestSignatureContent(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		specMessageID+".1614265330."+specBody,
		webhook.SignatureContent(specMessageID, specTimestamp, specBody),
	)
}

func TestSignMatchesTheStandardWebhooksVector(t *testing.T) {
	t.Parallel()

	content := webhook.SignatureContent(specMessageID, specTimestamp, specBody)

	signature, err := webhook.Sign(specSecret, content)
	require.NoError(t, err)
	assert.Equal(t, specSignature, signature,
		"signature must match the published Standard Webhooks vector")
}

func TestSigningKeyDecodesTheSecret(t *testing.T) {
	t.Parallel()

	key, err := webhook.SigningKey(specSecret)
	require.NoError(t, err)

	expected, decodeErr := base64.StdEncoding.DecodeString(
		strings.TrimPrefix(specSecret, webhook.SigningPrefix),
	)
	require.NoError(t, decodeErr)
	assert.Equal(t, expected, key, "the HMAC key is the decoded secret, not its characters")
}

func TestSigningKeyAcceptsAnUnprefixedSecret(t *testing.T) {
	t.Parallel()

	// Consumer libraries tolerate a secret pasted without its prefix; so does
	// the signer, otherwise the two sides would disagree about the same value.
	withPrefix, err := webhook.SigningKey(webhook.SigningPrefix + unprefixedB64)
	require.NoError(t, err)

	without, err := webhook.SigningKey(unprefixedB64)
	require.NoError(t, err)

	assert.Equal(t, withPrefix, without)
}

func TestSigningKeyRejectsUndecodableSecret(t *testing.T) {
	t.Parallel()

	_, err := webhook.SigningKey(invalidSecret)
	require.Error(t, err, "a secret that cannot decode must fail loudly, never sign with its literal bytes")
}

func TestSignatureHeaderSingleSecret(t *testing.T) {
	t.Parallel()

	header, err := webhook.SignatureHeader(
		[]string{specSecret}, specMessageID, specTimestamp, specBody,
	)
	require.NoError(t, err)
	assert.Equal(t, "v1,"+specSignature, header)
}

func TestSignatureHeaderCarriesEverySecretDuringRotation(t *testing.T) {
	t.Parallel()

	header, err := webhook.SignatureHeader(
		[]string{specSecret, otherSecret},
		specMessageID, specTimestamp, specBody,
	)
	require.NoError(t, err)

	entries := strings.Split(header, " ")
	require.Len(t, entries, 2, "one entry per usable secret, space-delimited")
	assert.Equal(t, "v1,"+specSignature, entries[0],
		"the first entry still verifies against the old secret")
	assert.NotEqual(t, entries[0], entries[1], "each secret contributes its own signature")
	assert.True(t, strings.HasPrefix(entries[1], "v1,"))
}

func TestSignatureHeaderWithoutSecretsIsEmpty(t *testing.T) {
	t.Parallel()

	header, err := webhook.SignatureHeader(nil, specMessageID, specTimestamp, specBody)
	require.NoError(t, err)
	assert.Empty(t, header)
}

func TestSignatureHeaderPropagatesAnUndecodableSecret(t *testing.T) {
	t.Parallel()

	_, err := webhook.SignatureHeader(
		[]string{specSecret, invalidSecret}, specMessageID, specTimestamp, specBody,
	)
	require.Error(t, err, "one bad secret must fail the delivery, not ship a partial header")
}

func TestSignatureChangesWithEveryInput(t *testing.T) {
	t.Parallel()

	sign := func(secret, deliveryID string, timestamp int64, body string) string {
		t.Helper()
		signature, err := webhook.Sign(secret, webhook.SignatureContent(deliveryID, timestamp, body))
		require.NoError(t, err)

		return signature
	}

	base := sign(specSecret, specMessageID, specTimestamp, specBody)

	assert.NotEqual(t, base, sign(otherSecret, specMessageID, specTimestamp, specBody),
		"a different secret must produce a different signature")
	assert.NotEqual(t, base, sign(specSecret, specMessageID, specTimestamp+1, specBody),
		"the timestamp is covered by the signature")
	assert.NotEqual(t, base, sign(specSecret, otherDelivery, specTimestamp, specBody),
		"the delivery id is covered by the signature")
	assert.NotEqual(t, base, sign(specSecret, specMessageID, specTimestamp, specBody+" "),
		"the body is covered byte-for-byte")
}

func TestGeneratedSecretShape(t *testing.T) {
	t.Parallel()

	secret, err := webhook.GenerateSecret()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(secret, webhook.SigningPrefix),
		"consumer libraries key off the whsec_ prefix")

	key, err := webhook.SigningKey(secret)
	require.NoError(t, err)
	assert.Len(t, key, webhook.SecretRandomBytes,
		"a generated secret must decode to the full entropy it claims")

	other, err := webhook.GenerateSecret()
	require.NoError(t, err)
	assert.NotEqual(t, secret, other)
}
