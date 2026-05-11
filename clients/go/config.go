package nanoclient

import (
	"context"
	"errors"
	"net/http"
)

// Config defines the parameters used to create a Anchor API client.
type Config struct {
	BaseURL        string
	Token          string
	HTTPClient     HttpRequestDoer
	RequestEditors []RequestEditorFn
}

// NewClientWithConfig creates a ClientWithResponses using the provided configuration.
func NewClientWithConfig(cfg Config, opts ...ClientOption) (*ClientWithResponses, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("base URL is required")
	}

	options := make([]ClientOption, 0, len(opts)+2)
	if cfg.HTTPClient != nil {
		options = append(options, WithHTTPClient(cfg.HTTPClient))
	}

	for _, editor := range cfg.RequestEditors {
		options = append(options, WithRequestEditorFn(editor))
	}

	if cfg.Token != "" {
		token := cfg.Token
		options = append(options, WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+token)
			return nil
		}))
	}

	options = append(options, opts...)

	return NewClientWithResponses(cfg.BaseURL, options...)
}
