package events

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	svix "github.com/svix/svix-webhooks/go"
)

const (
	headerWebhookID        = "Webhook-Id"
	headerWebhookTimestamp = "Webhook-Timestamp"
	headerWebhookSignature = "Webhook-Signature"
	signingSecretPrefix    = "whsec_"
	signingSecretBytes     = 32
	signedHeaderCount      = 3
)

func NewSigningSecret() (string, error) {
	key := make([]byte, signingSecretBytes)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("events: generate signing secret: %w", err)
	}
	return signingSecretPrefix + base64.StdEncoding.EncodeToString(key), nil
}

func Sign(secret, msgID string, timestamp time.Time, body []byte) (http.Header, error) {
	wh, err := svix.NewWebhook(secret)
	if err != nil {
		return nil, fmt.Errorf("events: signing secret: %w", err)
	}
	signature, err := wh.Sign(msgID, timestamp, body)
	if err != nil {
		return nil, fmt.Errorf("events: sign: %w", err)
	}
	headers := make(http.Header, signedHeaderCount)
	headers.Set(headerWebhookID, msgID)
	headers.Set(headerWebhookTimestamp, strconv.FormatInt(timestamp.Unix(), 10))
	headers.Set(headerWebhookSignature, signature)
	return headers, nil
}

func Verify(secret string, headers http.Header, body []byte) error {
	wh, err := svix.NewWebhook(secret)
	if err != nil {
		return fmt.Errorf("events: signing secret: %w", err)
	}
	if verifyErr := wh.Verify(body, headers); verifyErr != nil {
		return fmt.Errorf("events: verify: %w", verifyErr)
	}
	return nil
}
