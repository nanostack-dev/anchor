package itdsl

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	eventIDPrefix           = "pevt_"
	webhookIDHeader         = "Webhook-Id"
	webhookTimestampHeader  = "Webhook-Timestamp"
	webhookSignatureHeader  = "Webhook-Signature"
	webhookSignatureVersion = "v1,"
	signingSecretPrefix     = "whsec_"
	signingSecretBytes      = 32
	webhookTimestampSkew    = 5 * time.Minute
)

type StandardWebhookDelivery struct {
	Method  string
	Headers http.Header
	Body    []byte
}

func AssertStandardWebhook(t *testing.T, secret string, delivery StandardWebhookDelivery) {
	t.Helper()
	require.Equal(t, http.MethodPost, delivery.Method)
	require.Equal(t, "application/json", delivery.Headers.Get("Content-Type"))

	msgID := delivery.Headers.Get(webhookIDHeader)
	timestampHeader := delivery.Headers.Get(webhookTimestampHeader)
	signatureHeader := delivery.Headers.Get(webhookSignatureHeader)
	require.NotEmpty(t, msgID)
	require.NotEmpty(t, timestampHeader)
	require.NotEmpty(t, signatureHeader)
	require.True(t, strings.HasPrefix(msgID, eventIDPrefix), "webhook-id %q", msgID)

	unixSeconds, err := strconv.ParseInt(timestampHeader, 10, 64)
	require.NoError(t, err)
	headerTime := time.Unix(unixSeconds, 0)
	skew := time.Since(headerTime)
	require.LessOrEqual(t, skew, webhookTimestampSkew)
	require.GreaterOrEqual(t, skew, -webhookTimestampSkew)

	require.True(
		t,
		standardWebhookSignatureMatches(secret, delivery.Headers, delivery.Body),
		"signature must match the minted secret over Webhook-Id, Webhook-Timestamp, and body",
	)
	require.False(
		t,
		standardWebhookSignatureMatches(otherSigningSecret(), delivery.Headers, delivery.Body),
		"signature must not match a different secret",
	)
	tampered := append(append([]byte{}, delivery.Body...), 'x')
	require.False(
		t,
		standardWebhookSignatureMatches(secret, delivery.Headers, tampered),
		"signature must not match a tampered body",
	)

	var envelope struct {
		Type      string            `json:"type"`
		Timestamp string            `json:"timestamp"`
		Data      map[string]string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(delivery.Body, &envelope))
	require.NotEmpty(t, envelope.Type)
	_, timestampErr := time.Parse(time.RFC3339Nano, envelope.Timestamp)
	if timestampErr != nil {
		_, timestampErr = time.Parse(time.RFC3339, envelope.Timestamp)
	}
	require.NoError(t, timestampErr)
	require.NotNil(t, envelope.Data)
}

func standardWebhookSignatureMatches(secret string, headers http.Header, body []byte) bool {
	msgID := headers.Get(webhookIDHeader)
	timestampHeader := headers.Get(webhookTimestampHeader)
	signatures := headers.Get(webhookSignatureHeader)
	if msgID == "" || timestampHeader == "" || signatures == "" {
		return false
	}
	unixSeconds, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return false
	}
	key, err := decodeSigningSecret(secret)
	if err != nil {
		return false
	}
	expected := signStandardWebhook(key, msgID, unixSeconds, body)
	for candidate := range strings.SplitSeq(signatures, " ") {
		if hmac.Equal([]byte(candidate), []byte(expected)) {
			return true
		}
	}
	return false
}

func signStandardWebhook(key []byte, msgID string, unixSeconds int64, body []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = fmt.Fprintf(mac, "%s.%d.", msgID, unixSeconds)
	_, _ = mac.Write(body)
	return webhookSignatureVersion + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func decodeSigningSecret(secret string) ([]byte, error) {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return nil, errors.New("empty secret")
	}
	payload := strings.TrimPrefix(trimmed, signingSecretPrefix)
	key, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(key) == 0 {
		return nil, errors.New("invalid secret")
	}
	return key, nil
}

func otherSigningSecret() string {
	return signingSecretPrefix + base64.StdEncoding.EncodeToString(
		bytes.Repeat([]byte{0x42}, signingSecretBytes),
	)
}
