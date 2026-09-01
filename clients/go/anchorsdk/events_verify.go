package anchorsdk

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	webhookIDHeader        = "Webhook-Id"
	webhookTimestampHeader = "Webhook-Timestamp"
	webhookSignatureHeader = "Webhook-Signature"
	signingSecretPrefix    = "whsec_"
	signatureVersionPrefix = "v1,"
	webhookTimestampSkew   = 5 * time.Minute
)

func decodeSigningSecret(secret string) ([]byte, error) {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return nil, ErrWebhookSecret
	}
	payload := strings.TrimPrefix(trimmed, signingSecretPrefix)
	key, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(key) == 0 {
		return nil, ErrWebhookSecret
	}
	return key, nil
}

func signWebhook(secret []byte, msgID string, timestamp time.Time, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = fmt.Fprintf(mac, "%s.%d.", msgID, timestamp.Unix())
	_, _ = mac.Write(body)
	return signatureVersionPrefix + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func verifySignature(secret []byte, headers http.Header, body []byte) (string, error) {
	msgID := headers.Get(webhookIDHeader)
	timestampHeader := headers.Get(webhookTimestampHeader)
	signatures := headers.Get(webhookSignatureHeader)
	if msgID == "" || timestampHeader == "" || signatures == "" {
		return "", ErrWebhookSignature
	}
	unixSeconds, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return "", ErrWebhookSignature
	}
	timestamp := time.Unix(unixSeconds, 0)
	if skew := time.Since(timestamp); skew > webhookTimestampSkew || skew < -webhookTimestampSkew {
		return "", ErrWebhookSignature
	}
	expected := signWebhook(secret, msgID, timestamp, body)
	for candidate := range strings.SplitSeq(signatures, " ") {
		if hmac.Equal([]byte(candidate), []byte(expected)) {
			return msgID, nil
		}
	}
	return "", ErrWebhookSignature
}
