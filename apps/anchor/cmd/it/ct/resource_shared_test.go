package ct_test

import (
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/internal/authz/fga"
	"anchor/internal/authz/model"
)

// resourceWorld is a product with the document tree of ADR-0022 and one
// Organization to register resources in.
type resourceWorld struct {
	product   *itdsl.ProductContext
	tree      documentTree
	org       ct.ProductOrganizationResponse
	resources itdsl.ResourceClient
}

func givenResourceWorld(t *testing.T) resourceWorld {
	t.Helper()
	product := createTestProductContext(t)
	tree := declareDocumentTree(t, product)
	org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
	return resourceWorld{product: product, tree: tree, org: org, resources: product.Resources(org.Id)}
}

// givenNestingWorld is a product with one type, node, that sits under the
// Organization and nests in itself, so a chain can grow to any depth.
func givenNestingWorld(t *testing.T) resourceWorld {
	t.Helper()
	product := createTestProductContext(t)
	product.ResourceTypes().Create(ct.CreateProductResourceTypeJSONRequestBody{
		Slug: "node", Name: "Node", ParentTypes: []string{"organization"}, NestsInItself: new(true),
	})
	org := product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
	return resourceWorld{product: product, org: org, resources: product.Resources(org.Id)}
}

// resourceID is a fresh product identifier.
func resourceID() string {
	return ids.MustNew("ext")
}

// chainOf registers n nodes, each under the previous one, the first under the
// Organization, and returns their identifiers from the top.
func chainOf(t *testing.T, w resourceWorld, n int) []string {
	t.Helper()
	chain := make([]string, n)
	parent := w.resources.Top()
	for i := range n {
		chain[i] = resourceID()
		w.resources.Put("node", chain[i], "Node", parent)
		parent = itdsl.Ref("node", chain[i])
	}
	return chain
}

// resourceRowID reads Anchor's row ID of a registered resource.
func resourceRowID(t *testing.T, organizationID, externalID string) string {
	t.Helper()
	var rowID string
	require.NoError(t, testDB.QueryRowContext(t.Context(),
		`SELECT id FROM resources WHERE organization_id = $1 AND external_id = $2`,
		organizationID, externalID).Scan(&rowID))
	return rowID
}

func countResources(t *testing.T, column, value string) int {
	t.Helper()
	var count int
	require.NoError(t, testDB.QueryRowContext(t.Context(),
		`SELECT count(*) FROM resources WHERE `+column+` = $1`, value).Scan(&count))
	return count
}

// storedParents reads the parent tuples the product's store holds on an
// object.
func storedParents(t *testing.T, productID, object string) []string {
	t.Helper()
	found, err := authzStores.Open(t.Context(), productID)
	require.NoError(t, err)
	opened, ok := found.Get()
	if !ok {
		return nil
	}
	tuples, err := opened.Store.Read(t.Context(), fga.Filter{Object: object, Relation: model.RelationParent})
	require.NoError(t, err)
	var parents []string
	for _, tuple := range tuples {
		parents = append(parents, tuple.User)
	}
	return parents
}
