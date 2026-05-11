package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

func bearerTokenEditor(token string) nanoclient.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}
}

func productAPIKeyEditor(apiKey string) nanoclient.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("X-Product-Api-Key", apiKey)
		return nil
	}
}

func formatAPIError(operation string, statusCode int, body []byte) string {
	bodyText := strings.TrimSpace(string(body))
	if bodyText == "" {
		return fmt.Sprintf("%s failed with status %d", operation, statusCode)
	}

	return fmt.Sprintf("%s failed with status %d: %s", operation, statusCode, bodyText)
}

func stringPtrFromTFValue(value string) *string {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	return &v
}
