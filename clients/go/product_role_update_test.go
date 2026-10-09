package nanoclient_test

import (
	"encoding/json"
	"testing"

	anchor "github.com/nanostack-dev/anchor/clients/go"
)

func TestProductRoleUpdatePermissionJSON(t *testing.T) {
	t.Parallel()
	empty := []string{}
	replacement := []string{"document:read"}
	for _, test := range []struct {
		name        string
		permissions *[]string
		expected    string
	}{
		{"omitted", nil, `{"name":"Reader"}`},
		{"empty", &empty, `{"name":"Reader","permissions":[]}`},
		{"replacement", &replacement, `{"name":"Reader","permissions":["document:read"]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := json.Marshal(anchor.UpdateProductRoleJSONRequestBody{Name: "Reader", Permissions: test.permissions})
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != test.expected {
				t.Fatalf("got %s; want %s", encoded, test.expected)
			}
		})
	}
}
