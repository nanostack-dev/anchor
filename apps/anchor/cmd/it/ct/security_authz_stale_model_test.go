package ct_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Decisions come from OpenFGA (PR 278) and read whatever the Product's store
// holds; nothing checks that the store still follows the tables. Once the
// Product's catalog compiles to a model OpenFGA refuses (here: over its 1 MiB
// model limit, reached with ~200 permissions on a wide type tree), every sync
// of the Product fails on ensureModel, the replay only logs it, and decisions
// keep answering from the last model and the last tuples. A member moved to a
// role without a permission then keeps that permission for good.
func TestSecurityDecisionsWhenTheStoreCannotFollowTheTables(t *testing.T) {
	t.Parallel()
	t.Run("A member moved off a role is refused its permission even when the store can no longer be synced",
		func(t *testing.T) {
			t.Parallel()
			w := newRoleTemplateWorld(t)
			writer := createRoleTemplate(t, w.product,
				ct.CreateProductRoleJSONRequestBody{Name: "Writer", Permissions: []string{"file:write"}})
			user := createDSLProductUser(t, w.product)
			external := "user_" + uuid.NewString()
			setDSLProductUserExternalID(t, w.product, user.ID, external)
			addMember(t, w.product, w.organization.Id, user.ID, w.admin.Id)
			t.Cleanup(func() { deleteOrganizationQuietly(t, w) })
			question := w.product.Access().User(external).InOrganization(w.organization.Id).Can("file:read")
			require.True(t, question.Decide().Decision)

			growCatalogPastTheModelLimit(t, w)

			resp, err := w.product.AllScopeAPIKeyClient().UpdateOrganizationMemberRoleWithResponse(
				t.Context(), w.product.ProductID, w.organization.Id, user.ID,
				ct.UpdateOrganizationMemberRoleJSONRequestBody{RoleId: writer.Id})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))

			decision := question.Decide()
			assert.False(t, decision.Decision, "the member no longer holds file:read in Anchor's tables")
		})
}

// growCatalogPastTheModelLimit declares a wide tree of resource types, scopes
// 210 permissions to a leaf type under the organization, then moves the leaf
// under ten types that each sit under ten more: every permission is then held
// on about thirty types, and the compiled model passes 1 MiB.
func growCatalogPastTheModelLimit(t *testing.T, w roleTemplateWorld) {
	t.Helper()
	types := w.product.ResourceTypes()
	var tree []ct.ProductResourceTypeTreeItem
	parents := []ct.ResourceTypeSlug{"organization"}
	for level := 1; level <= 3; level++ {
		var next []ct.ResourceTypeSlug
		for i := range 10 {
			slug := ct.ResourceTypeSlug(fmt.Sprintf("l%d_%d", level, i))
			tree = append(tree, ct.ProductResourceTypeTreeItem{Slug: slug, Name: string(slug), ParentTypes: parents})
			next = append(next, slug)
		}
		parents = next
	}
	leaf := ct.ProductResourceTypeTreeItem{Slug: "leaf", Name: "leaf", ParentTypes: []ct.ResourceTypeSlug{"organization"}}
	types.ReplaceTree(append(tree, leaf)...)
	names := make([]string, 210)
	for i := range names {
		names[i] = fmt.Sprintf("leaf:action_%03d", i)
	}
	w.product.CreateScopedPermissions(t, "leaf", names...)
	leaf.ParentTypes = parents
	types.ReplaceTree(append(tree, leaf)...)
}

// deleteOrganizationQuietly removes the test's Organization, so the suite's
// mirror check skips a Product whose store this test leaves behind on purpose.
func deleteOrganizationQuietly(t *testing.T, w roleTemplateWorld) {
	t.Helper()
	_, _ = w.product.AllScopeAPIKeyClient().DeleteProductOrganizationWithResponse(
		context.Background(), w.product.ProductID, w.organization.Id)
}
