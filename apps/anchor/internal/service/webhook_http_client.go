package service

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"anchor/internal/domain/webhook"
	"anchor/internal/service/config"
)

const (
	// webhookRequestTimeout is the total budget for one delivery attempt,
	// connection through body read. A hostile receiver can otherwise stall a
	// worker indefinitely.
	webhookRequestTimeout = 15 * time.Second

	webhookDialTimeout          = 5 * time.Second
	webhookTLSHandshakeTimeout  = 5 * time.Second
	webhookIdleConnTimeout      = 30 * time.Second
	webhookMaxIdleConns         = 32
	webhookMaxIdleConnsPerHost  = 4
	webhookResponseHeaderWindow = 10 * time.Second
	webhookUserAgent            = "Anchor-Webhooks/1.0"
)

// WebhookHTTPClient is the dedicated, SSRF-hardened client used for outbound
// webhook delivery. It is deliberately separate from any other HTTP client in
// the process: the target URL is supplied by a product administrator, and
// Anchor sits inside the same network as internal services.
type WebhookHTTPClient struct {
	client        *http.Client
	allowInsecure bool
}

// WebhookHTTPResponse is the trimmed result of one delivery attempt.
type WebhookHTTPResponse struct {
	StatusCode int
	// Snippet is the first MaxResponseSnippetBytes of the response body.
	Snippet string
	// RetryAfter is the receiver's Retry-After header, when present.
	RetryAfter string
	Duration   time.Duration
}

// NewWebhookHTTPClient builds the delivery client.
//
// The control that actually works against SSRF is net.Dialer.Control: it fires
// after DNS resolution with the literal ip:port about to be dialed, so DNS
// rebinding cannot slip a private address past a hostname check made earlier.
// Alongside it, redirects are never followed — a 302 to 127.0.0.1 is the oldest
// trick here — and the response body read is capped.
func NewWebhookHTTPClient(coreCfg *config.CoreConfig) *WebhookHTTPClient {
	allowInsecure := coreCfg != nil && coreCfg.Webhooks.AllowInsecureTargets

	dialer := &net.Dialer{
		Timeout: webhookDialTimeout,
		Control: func(_ string, address string, _ syscall.RawConn) error {
			if allowInsecure {
				return nil
			}
			if webhook.IsBlockedAddress(address) {
				return webhook.ErrBlockedTarget
			}

			return nil
		},
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   webhookTLSHandshakeTimeout,
		IdleConnTimeout:       webhookIdleConnTimeout,
		MaxIdleConns:          webhookMaxIdleConns,
		MaxIdleConnsPerHost:   webhookMaxIdleConnsPerHost,
		ResponseHeaderTimeout: webhookResponseHeaderWindow,
		ForceAttemptHTTP2:     true,
	}

	return &WebhookHTTPClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   webhookRequestTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				// Stop at the redirect and hand the 3xx back as the response.
				// Classification then treats it as a permanent endpoint
				// misconfiguration, which records a far more useful attempt row
				// than a synthetic transport error would.
				return http.ErrUseLastResponse
			},
		},
		allowInsecure: allowInsecure,
	}
}

// AllowsInsecureTargets reports whether the relaxed target policy is active.
func (c *WebhookHTTPClient) AllowsInsecureTargets() bool {
	return c.allowInsecure
}

// Post sends one signed delivery attempt and returns the trimmed response.
func (c *WebhookHTTPClient) Post(
	ctx context.Context, targetURL string, body string, headers map[string]string,
) (WebhookHTTPResponse, error) {
	started := time.Now()

	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, targetURL, strings.NewReader(body),
	)
	if err != nil {
		return WebhookHTTPResponse{Duration: time.Since(started)}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", webhookUserAgent)
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	response, err := c.client.Do(request)
	if err != nil {
		return WebhookHTTPResponse{Duration: time.Since(started)}, err
	}
	defer func() {
		_ = response.Body.Close()
	}()

	capped, readErr := io.ReadAll(io.LimitReader(response.Body, webhook.MaxResponseReadBytes))
	if readErr != nil {
		return WebhookHTTPResponse{
			StatusCode: response.StatusCode,
			RetryAfter: response.Header.Get("Retry-After"),
			Duration:   time.Since(started),
		}, readErr
	}

	return WebhookHTTPResponse{
		StatusCode: response.StatusCode,
		Snippet:    webhook.TruncateSnippet(capped),
		RetryAfter: response.Header.Get("Retry-After"),
		Duration:   time.Since(started),
	}, nil
}
