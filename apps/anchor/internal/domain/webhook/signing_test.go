package webhook_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/webhook"
)

// Known vector for Anchor's signing scheme. The header layout, the
// `{id}.{timestamp}.{body}` signing content and the `v1,<base64>` entry format
// follow Standard Webhooks verbatim; the one deviation is the key material.
// Anchor's secrets are checksummed prefixed tokens rather than base64 payloads,
// so the HMAC key is the UTF-8 bytes of the secret exactly as handed to the
// customer, with no base64 decode step.
const (
	vectorSecret     = "anchor_whsec_ULRIzWxErrpEjHOoRKgSNvhVwtVYWlbEuNwjKjTAmiUhBvzu_1a2b3c4d"
	vectorSecondary  = "anchor_whsec_SECONDSECRETSECONDSECRETSECONDSECRETSECONDSE_9f8e7d6c"
	vectorDeliveryID = "whd_2iABCDEFGHIJKLMNOPQRSTUVW"
	vectorTimestamp  = int64(1753280531)
	vectorBody       = `{"id":"evt_2gH","type":"ping"}`
	vectorSignature  = "VXEvxddZrVt5Q7iFTzynRM/cKdOkGJVt9Ub41MwuUu4="
	vectorSignature2 = "jdllapX5seUS6IEwbQKe+hn+xtPqFCNYWkHj9XtLMQ4="
)

func TestSignatureContent(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		"whd_2iABCDEFGHIJKLMNOPQRSTUVW.1753280531."+vectorBody,
		webhook.SignatureContent(vectorDeliveryID, vectorTimestamp, vectorBody),
	)
}

func TestSignMatchesTheKnownVector(t *testing.T) {
	t.Parallel()

	content := webhook.SignatureContent(vectorDeliveryID, vectorTimestamp, vectorBody)
	assert.Equal(t, vectorSignature, webhook.Sign(vectorSecret, content))
}

func TestSignatureHeaderSingleSecret(t *testing.T) {
	t.Parallel()

	header := webhook.SignatureHeader(
		[]string{vectorSecret}, vectorDeliveryID, vectorTimestamp, vectorBody,
	)
	assert.Equal(t, "v1,"+vectorSignature, header)
}

func TestSignatureHeaderCarriesEverySecretDuringRotation(t *testing.T) {
	t.Parallel()

	header := webhook.SignatureHeader(
		[]string{vectorSecret, vectorSecondary},
		vectorDeliveryID, vectorTimestamp, vectorBody,
	)

	entries := strings.Split(header, " ")
	require.Len(t, entries, 2, "one entry per usable secret, space-delimited")
	assert.Equal(t, "v1,"+vectorSignature, entries[0])
	assert.Equal(t, "v1,"+vectorSignature2, entries[1])
}

func TestSignatureHeaderWithoutSecretsIsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(
		t, webhook.SignatureHeader(nil, vectorDeliveryID, vectorTimestamp, vectorBody),
	)
}

func TestSignatureChangesWithEveryInput(t *testing.T) {
	t.Parallel()

	base := webhook.Sign(
		vectorSecret,
		webhook.SignatureContent(vectorDeliveryID, vectorTimestamp, vectorBody),
	)

	assert.NotEqual(t, base, webhook.Sign(
		vectorSecondary,
		webhook.SignatureContent(vectorDeliveryID, vectorTimestamp, vectorBody),
	), "a different secret must produce a different signature")

	assert.NotEqual(t, base, webhook.Sign(
		vectorSecret,
		webhook.SignatureContent(vectorDeliveryID, vectorTimestamp+1, vectorBody),
	), "the timestamp is covered by the signature")

	assert.NotEqual(t, base, webhook.Sign(
		vectorSecret,
		webhook.SignatureContent("whd_other", vectorTimestamp, vectorBody),
	), "the delivery id is covered by the signature")

	assert.NotEqual(t, base, webhook.Sign(
		vectorSecret,
		webhook.SignatureContent(vectorDeliveryID, vectorTimestamp, vectorBody+" "),
	), "the body is covered byte-for-byte")
}

func TestGeneratedSecretShape(t *testing.T) {
	t.Parallel()

	secret, err := webhook.GenerateSecret()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(secret, webhook.SigningPrefix))
	assert.True(t, webhook.SecretSpec().Validate(secret))

	other, err := webhook.GenerateSecret()
	require.NoError(t, err)
	assert.NotEqual(t, secret, other)
}
