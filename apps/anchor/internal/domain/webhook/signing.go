package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
)

// Standard Webhooks header names. Following the spec verbatim means every
// consumer verifies with an off-the-shelf library instead of reading our docs.
const (
	HeaderWebhookID        = "webhook-id"
	HeaderWebhookTimestamp = "webhook-timestamp"
	HeaderWebhookSignature = "webhook-signature"
)

// SignatureVersion is the symmetric HMAC-SHA256 scheme identifier. The spec
// reserves `v1a,` for ed25519, so adding opt-in asymmetric signing per endpoint
// later is additive rather than breaking.
const SignatureVersion = "v1"

// SignatureContent is the signed payload: `{webhook-id}.{webhook-timestamp}.{body}`.
//
// The delivery id is stable across retries so a receiver can use it as an
// idempotency key; the timestamp is fresh per attempt so a replayed capture
// falls outside the receiver's tolerance window.
func SignatureContent(deliveryID string, timestamp int64, body string) string {
	return deliveryID + "." + strconv.FormatInt(timestamp, 10) + "." + body
}

// Sign returns the base64 HMAC-SHA256 of content under secret.
//
// The HMAC key is the secret's decoded bytes, not its literal characters — see
// SigningKey. Signing with the literal string would produce a signature no
// standard verifier accepts, so a secret that does not decode is an error
// rather than a silently wrong signature.
func Sign(secret, content string) (string, error) {
	key, err := SigningKey(secret)
	if err != nil {
		return "", err
	}

	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(content))

	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// SignatureHeader builds the space-delimited `webhook-signature` value: one
// `v1,<signature>` entry per usable secret, so both sides of a rotation verify.
func SignatureHeader(
	plaintextSecrets []string, deliveryID string, timestamp int64, body string,
) (string, error) {
	if len(plaintextSecrets) == 0 {
		return "", nil
	}

	content := SignatureContent(deliveryID, timestamp, body)
	entries := make([]string, 0, len(plaintextSecrets))
	for _, secret := range plaintextSecrets {
		signature, err := Sign(secret, content)
		if err != nil {
			return "", err
		}
		entries = append(entries, SignatureVersion+","+signature)
	}

	return strings.Join(entries, " "), nil
}
