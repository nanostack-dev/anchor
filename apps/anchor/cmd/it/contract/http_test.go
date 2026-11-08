//go:build contract

package contract_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	requestTimeout = 60 * time.Second
	userAgent      = "anchor-contract/1"
)

var httpClient = &http.Client{
	Timeout: requestTimeout,
	// The suite asserts redirects as responses, never follows them.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type response struct {
	status int
	header http.Header
	body   []byte
	parsed any
	isJSON bool
}

// json returns the body as an object, or an empty map when it is not one.
func (r *response) json() map[string]any {
	if obj, ok := r.parsed.(map[string]any); ok {
		return obj
	}
	return map[string]any{}
}

// rawDo sends one request with the given headers and raw body. An empty body
// sends no body at all. requestID becomes the X-Request-Id header, which the
// server reuses as the request_id log field.
func rawDo(
	ctx context.Context, method, url string, headers map[string]string, body, requestID string,
) (*response, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	out := &response{status: resp.StatusCode, header: resp.Header, body: raw}
	if len(raw) > 0 {
		var parsed any
		if json.Unmarshal(raw, &parsed) == nil {
			out.parsed = parsed
			out.isJSON = true
		}
	}
	return out, nil
}
