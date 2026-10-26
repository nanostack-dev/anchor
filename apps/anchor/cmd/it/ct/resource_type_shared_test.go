package ct_test

import (
	"encoding/json"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

type statusResponse interface {
	StatusCode() int
}

// requireAPIError asserts the status and the one error code of a refusal, and
// returns the error's metadata.
func requireAPIError(t *testing.T, resp statusResponse, body []byte, status int, code string) map[string]any {
	t.Helper()
	require.Equal(t, status, resp.StatusCode(), string(body))
	var decoded ct.ApiErrorResponse
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Len(t, decoded.Errors, 1, string(body))
	require.Equal(t, code, decoded.Errors[0].Code, string(body))
	if decoded.Errors[0].Metadata == nil {
		return map[string]any{}
	}
	return *decoded.Errors[0].Metadata
}

// documentTree declares the tree ADR-0022 walks through: project under
// organization, folder under project and nesting in itself, and document
// under folder or at the project root.
type documentTree struct {
	project, folder, document ct.ProductResourceTypeResponse
}

func declareDocumentTree(t *testing.T, product *itdsl.ProductContext) documentTree {
	t.Helper()
	types := product.ResourceTypes()
	project := types.CreateUnder("project", "organization")
	folder := types.Create(ct.CreateProductResourceTypeJSONRequestBody{
		Slug: "folder", Name: "Folder", ParentTypes: []string{"project"}, NestsInItself: new(true),
	})
	document := types.CreateUnder("document", "folder", "project")
	return documentTree{project: project, folder: folder, document: document}
}
