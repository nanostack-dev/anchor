package ct_test

import (
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// memberWorld is a product with the document tree, folder and project roles
// it manages, an Organization holding a project with one folder, and a member
// of the Organization.
type memberWorld struct {
	product   *itdsl.ProductContext
	org       ct.ProductOrganizationResponse
	resources itdsl.ResourceClient
	// project and folder are the product's identifiers.
	project, folder string
	// The roles the product manages.
	orgMember, editor, viewer, projectAdmin ct.ProductRoleResponse
	member                                  string
}

func givenMemberWorld(t *testing.T) memberWorld {
	t.Helper()
	product := createTestProductContext(t)
	declareDocumentTree(t, product)
	w := memberWorld{
		product:      product,
		orgMember:    createManagedRole(t, product, roleOn("organization", "Member")),
		editor:       createManagedRole(t, product, roleOn("folder", "Folder editor")),
		viewer:       createManagedRole(t, product, roleOn("folder", "Folder viewer")),
		projectAdmin: createManagedRole(t, product, roleOn("project", "Project admin")),
	}
	w.org = product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
	w.resources = product.Resources(w.org.Id)
	w.project, w.folder = resourceID(), resourceID()
	w.resources.Put("project", w.project, "Website redesign", w.resources.Top())
	w.resources.Put("folder", w.folder, "Design", itdsl.Ref("project", w.project))
	w.member = w.newMember(t)
	return w
}

func roleOn(resourceType, name string) ct.CreateProductRoleJSONRequestBody {
	return ct.CreateProductRoleJSONRequestBody{Name: name + " " + ids.MustNew("ct"), ResourceType: resourceType}
}

// newMember adds a fresh product user to the Organization with the
// Organization-scoped role.
func (w memberWorld) newMember(t *testing.T) string {
	t.Helper()
	user := createDSLProductUser(t, w.product)
	addMember(t, w.product, w.org.Id, user.ID, w.orgMember.Id)
	return user.ID
}

func createManagedRole(
	t *testing.T, product *itdsl.ProductContext, body ct.CreateProductRoleJSONRequestBody,
) ct.ProductRoleResponse {
	t.Helper()
	resp, err := product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(t.Context(), product.ProductID, body)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return *resp.JSON201
}

func (w memberWorld) folderMembers(scopes ...string) itdsl.ResourceMemberClient {
	return w.product.ResourceMembers(w.org.Id, "folder", w.folder, scopes...)
}

func (w memberWorld) projectMembers() itdsl.ResourceMemberClient {
	return w.product.ResourceMembers(w.org.Id, "project", w.project)
}

func roleIDs(member ct.ResourceMemberResponse) []string {
	ids := make([]string, len(member.Roles))
	for i, role := range member.Roles {
		ids[i] = role.Id
	}
	return ids
}

// countAssignments counts the assignment rows of one member in the
// Organization.
func countAssignments(t *testing.T, organizationID, productUserID string) int {
	t.Helper()
	var count int
	require.NoError(t, testDB.QueryRowContext(t.Context(),
		`SELECT count(*) FROM resource_role_assignments WHERE organization_id = $1 AND product_user_id = $2`,
		organizationID, productUserID).Scan(&count))
	return count
}

func addMember(t *testing.T, product *itdsl.ProductContext, organizationID, productUserID, roleID string) {
	t.Helper()
	resp, err := product.AllScopeAPIKeyClient().AddOrganizationMemberWithResponse(t.Context(), product.ProductID,
		organizationID, ct.AddOrganizationMemberJSONRequestBody{ProductUserId: productUserID, RoleId: &roleID})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
}
