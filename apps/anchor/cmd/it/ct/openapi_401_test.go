package ct_test

import (
	"io"
	"net/http"
	"testing"

	itshared "anchor/cmd/it/shared"
)

func TestOpenAPIProtected401(t *testing.T) {
	noAuth := map[string]struct{}{
		"post /v1/auth/login":    {},
		"post /v1/auth/logout":   {},
		"post /v1/auth/register": {},
		"post /v1/auth/refresh":  {},
	}

	spec, components, paramDefs := parseOpenAPISpec(t)
	for _, op := range extractOperations(spec, paramDefs, noAuth, itshared.ServerURL) {
		t.Run(
			op.OpKey, func(t *testing.T) {
				resp, err := sendRequest(op, components, nil)
				if err != nil {
					t.Fatalf("failed to %s %s: %v", op.Verb, op.URL, err)
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusUnauthorized {
					bodyBytes, _ := io.ReadAll(resp.Body)
					t.Errorf(
						"expected 401 Unauthorized for %s, got %d. Response body: %s", op.URL,
						resp.StatusCode, string(bodyBytes),
					)
				}
			},
		)
	}
}
