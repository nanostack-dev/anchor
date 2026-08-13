package anchorsdk

import (
	"context"
	"errors"
	"net/http"
	"time"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// productAuthHeader names the header Anchor authenticates product backends with.
// It is a header name, not a credential — the key itself comes from Config.
const productAuthHeader = "X-Product-Api-Key"

// defaultTimeout bounds a single request when the caller supplies no HTTP client.
const defaultTimeout = 10 * time.Second

// Config configures a [Client]. BaseURL, ProductID, and ProductAPIKey are
// required; everything else has a working default.
type Config struct {
	// BaseURL is the root of the Anchor API, e.g. "https://anchor.example.com".
	BaseURL string `yaml:"base_url"`
	// ProductID is the product this client acts on behalf of. It is bound once
	// here and never repeated at a call site.
	ProductID string `yaml:"product_id"`
	// ProductAPIKey authenticates every request, sent as X-Product-Api-Key.
	ProductAPIKey string `yaml:"product_api_key"`

	// HTTPClient overrides the transport. When nil, a client with a
	// 10s timeout is used.
	HTTPClient *http.Client `yaml:"-"`
	// Retry overrides the retry policy. The zero value uses the defaults.
	Retry RetryPolicy `yaml:"-"`
	// LicenseCache overrides how [License.Get] caches a license read. The
	// zero value uses the defaults; see [LicenseCachePolicy].
	LicenseCache LicenseCachePolicy `yaml:"-"`
}

func (c Config) validate() error {
	switch {
	case c.BaseURL == "":
		return errors.New("anchorsdk: BaseURL is required")
	case c.ProductID == "":
		return errors.New("anchorsdk: ProductID is required")
	case c.ProductAPIKey == "":
		return errors.New("anchorsdk: ProductAPIKey is required")
	default:
		return nil
	}
}

// Option customizes a [Client] beyond what [Config] exposes, by passing options
// straight through to the generated client.
type Option func(*options)

type options struct {
	client []nanoclient.ClientOption
}

// WithRequestEditor adds a request editor applied to every request, after the
// SDK's own authentication header. Use it for tracing headers or a tenant hint.
func WithRequestEditor(fn nanoclient.RequestEditorFn) Option {
	return func(o *options) {
		o.client = append(o.client, nanoclient.WithRequestEditorFn(fn))
	}
}

// WithClientOption passes a generated-client option through unchanged.
func WithClientOption(opt nanoclient.ClientOption) Option {
	return func(o *options) { o.client = append(o.client, opt) }
}

// Client is the product-scoped entry point to Anchor. It is safe for concurrent
// use and intended to be built once and shared.
//
// Reach features through its facades: [Client.Email], [Client.Organizations],
// [Client.Users], and [Client.Organization] for operations scoped to a single
// organization.
type Client struct {
	api       *nanoclient.ClientWithResponses
	productID string
	retry     RetryPolicy

	licenses      *licenseCache
	licensePolicy LicenseCachePolicy
}

// New builds a Client from cfg.
func New(cfg Config, opts ...Option) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	resolved := options{
		client: []nanoclient.ClientOption{
			nanoclient.WithHTTPClient(httpClient),
			nanoclient.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
				req.Header.Set(productAuthHeader, cfg.ProductAPIKey)
				return nil
			}),
		},
	}
	for _, opt := range opts {
		opt(&resolved)
	}

	api, err := nanoclient.NewClientWithResponses(cfg.BaseURL, resolved.client...)
	if err != nil {
		return nil, err
	}

	return &Client{
		api:           api,
		productID:     cfg.ProductID,
		retry:         cfg.Retry,
		licenses:      newLicenseCache(),
		licensePolicy: cfg.LicenseCache,
	}, nil
}

// ProductID returns the product this client is bound to.
func (c *Client) ProductID() string { return c.productID }

// Raw exposes the generated client for operations this SDK does not wrap —
// notably platform administration. Prefer a facade where one exists; anything
// reached through Raw loses the SDK's retry and error classification.
func (c *Client) Raw() *nanoclient.ClientWithResponses { return c.api }

// Organization returns a handle scoped to one organization, the entry point for
// its members, workspaces, and API keys.
//
//	org := c.Organization("org_3iXYZ")
//	members, err := org.Members().List(ctx)
func (c *Client) Organization(organizationID string) *Org {
	return &Org{c: c, id: organizationID}
}

// Org is a handle to a single organization. Obtain one with [Client.Organization].
type Org struct {
	c  *Client
	id string
}

// ID returns the organization this handle is bound to.
func (o *Org) ID() string { return o.id }
