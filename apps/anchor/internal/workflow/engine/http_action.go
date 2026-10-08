package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"anchor/internal/events"
)

const (
	httpActionTimeout     = 10 * time.Second
	maxHTTPResponseBytes  = 1 << 20
	maxHTTPErrorPreview   = 300
	httpSuccessStatusMin  = 200
	httpSuccessStatusMax  = 300
	keyURL                = "url"
	keyMethod             = "method"
	keyBody               = "body"
	httpMessageIDSplitter = ":"
)

var errPrivateAddress = errors.New("the address is private, loopback or link-local")

// SigningSecrets gives the secret a Product verifies Anchor's requests with:
// the signing secret of its event endpoint.
type SigningSecrets interface {
	DeliveryTarget(ctx context.Context, productID string) (events.DeliveryTarget, bool, error)
}

// HTTPCaller performs the requests of http.request steps. Unless private
// targets are allowed, it refuses plain HTTP and any address that is not
// public, checked on the address actually dialed so a DNS answer cannot
// redirect the call inward.
type HTTPCaller struct {
	client       *http.Client
	secrets      SigningSecrets
	allowPrivate bool
	now          func() time.Time
}

//nolint:gochecknoglobals // fixed list of non-public ranges
var nonPublicRanges = mustParseCIDRs(
	"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15",
	"198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "64:ff9b::/96", "64:ff9b:1::/48",
	"2001:db8::/32",
)

func mustParseCIDRs(cidrs ...string) []*net.IPNet {
	parsed := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		parsed = append(parsed, network)
	}
	return parsed
}

// IsPublicAddress reports whether a workflow step may call the address.
func IsPublicAddress(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	for _, network := range nonPublicRanges {
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

func NewHTTPCaller(secrets SigningSecrets, allowPrivate bool) *HTTPCaller {
	dialer := &net.Dialer{Timeout: httpActionTimeout}
	if !allowPrivate {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if !IsPublicAddress(net.ParseIP(host)) {
				return errPrivateAddress
			}
			return nil
		}
	}
	transport := &http.Transport{DialContext: dialer.DialContext, Proxy: nil}
	return &HTTPCaller{
		client: &http.Client{
			Timeout:   httpActionTimeout,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		secrets:      secrets,
		allowPrivate: allowPrivate,
		now:          time.Now,
	}
}

func (c *HTTPCaller) checkURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("%q is not an absolute HTTP or HTTPS URL", raw)
	}
	if !c.allowPrivate && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%q must use HTTPS", raw)
	}
	return parsed, nil
}

func (c *HTTPCaller) call(ctx context.Context, env Env, p Params) (map[string]any, error) {
	target, err := c.checkURL(p.String(keyURL))
	if err != nil {
		return nil, err
	}
	method := strings.ToUpper(p.String(keyMethod))
	if method == "" {
		method = http.MethodPost
	}
	body, err := c.body(env, p)
	if err != nil {
		return nil, err
	}
	secret, found, err := c.secrets.DeliveryTarget(ctx, env.ProductID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("configure the product's event endpoint first: its signing secret signs custom actions")
	}
	headers, err := events.Sign(
		secret.Secret, env.RunID+httpMessageIDSplitter+env.StepID, c.now(), body,
	)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header = headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(events.CausationHeader, events.EncodeCausationHeader(events.CausationFrom(ctx)))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", target.Host, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBytes))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < httpSuccessStatusMin || resp.StatusCode >= httpSuccessStatusMax {
		preview := string(raw)
		if len(preview) > maxHTTPErrorPreview {
			preview = preview[:maxHTTPErrorPreview] + "…"
		}
		return nil, fmt.Errorf("%s answered %d: %s", target.Host, resp.StatusCode, preview)
	}
	output := map[string]any{keyStatus: resp.StatusCode}
	var decoded any
	if len(raw) > 0 && json.Unmarshal(raw, &decoded) == nil {
		output[keyBody] = decoded
	} else {
		output[keyBody] = string(raw)
	}
	return output, nil
}

func (c *HTTPCaller) body(env Env, p Params) ([]byte, error) {
	object, given, err := p.Object(keyBody)
	if err != nil {
		return nil, err
	}
	if given {
		return json.Marshal(object)
	}
	return json.Marshal(map[string]any{
		"workflow_id": env.WorkflowID,
		"run_id":      env.RunID,
		"step_id":     env.StepID,
		keyEvent: map[string]any{
			"id": env.EventID, "type": env.EventType, keyData: env.EventData,
		},
	})
}

func httpRequestAction(caller *HTTPCaller) action {
	return action{
		spec: ActionSpec{
			Type: ActionHTTPRequest, Group: GroupCustom, Name: "Call your backend", Writes: true,
			Description: "Sends a signed request (Standard Webhooks, with your event endpoint's secret) " +
				"and gives later steps the JSON answer. Send back the Anchor-Workflow-Causation header " +
				"on the calls your backend makes to Anchor, so its writes stay inside this chain.",
			Params: []ParamSpec{
				{Name: keyURL, Label: "URL", Type: ParamURL, Required: true},
				{
					Name:  keyMethod,
					Label: "Method",
					Type:  ParamText,
					Options: []string{
						http.MethodPost,
						http.MethodPut,
						http.MethodPatch,
						http.MethodGet,
						http.MethodDelete,
					},
					Description: "POST when empty.",
				},
				{
					Name: keyBody, Label: "Body", Type: ParamJSON,
					Description: "JSON object to send. When empty, Anchor sends the run, step and event.",
				},
			},
			Outputs: []OutputSpec{
				{Name: keyStatus, Description: "HTTP status of the answer."},
				{Name: keyBody, Description: "JSON answer; read a field with body.<key>."},
			},
		},
		run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
			if caller == nil {
				return nil, errors.New("custom actions are not configured")
			}
			return caller.call(ctx, env, p)
		},
	}
}
