package ct_test

import (
	"context"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
)

// idorWorld is Acme's folder Design, which Dan holds folder-editor on, and
// everything that must never reach it: another Organization of the same
// Product, another Product of the same tenant, and another tenant.
type idorWorld struct {
	memberWorld
	// granter is the folder role Dan holds on Design; it grants folder:edit.
	granter ct.ProductRoleResponse
	danID   string
	// otherOrg is a second Organization of the same Product, with its own
	// member and a custom role on folder.
	otherOrg    ct.ProductOrganizationResponse
	otherMember string
	otherCustom ct.ProductRoleResponse
	ownCustom   ct.ProductRoleResponse
	// otherProduct belongs to the same tenant; otherTenantKey is the platform
	// session of a separate tenant.
	otherProduct   *itdsl.ProductContext
	otherKey       *ct.ClientWithResponses
	otherRole      ct.ProductRoleResponse
	otherUser      string
	otherTenantKey *ct.ClientWithResponses
}

func givenIDORWorld(t *testing.T) idorWorld {
	t.Helper()
	w := idorWorld{memberWorld: givenMemberWorld(t)}
	w.product.CreateScopedPermissions(t, "folder", "folder:edit")
	w.granter = createManagedRole(t, w.product, ct.CreateProductRoleJSONRequestBody{
		Name: "Folder granter " + ids.MustNew("ct"), ResourceType: "folder", Permissions: []string{"folder:edit"},
	})
	w.folderMembers().Put(w.member, w.granter.Id)
	w.danID = "user_" + ids.MustNew("ct")
	setDSLProductUserExternalID(t, w.product, w.member, w.danID)
	switchCustomRoles(t, w.product, "folder", true)

	w.otherOrg = w.product.CreateOrganization(t, "Org-"+ids.MustNew("ct"), nil)
	user := createDSLProductUser(t, w.product)
	addMember(t, w.product, w.otherOrg.Id, user.ID, w.orgMember.Id)
	w.otherMember = user.ID
	w.ownCustom = createCustomRole(t, w.product, w.org.Id, "folder")
	w.otherCustom = createCustomRole(t, w.product, w.otherOrg.Id, "folder")

	w.otherProduct = createTestProductContext(t)
	declareDocumentTree(t, w.otherProduct)
	w.otherKey = w.otherProduct.AllScopeAPIKeyClient()
	w.otherRole = createManagedRole(t, w.otherProduct, roleOn("folder", "Foreign editor"))
	w.otherUser = createDSLProductUser(t, w.otherProduct).ID
	w.otherTenantKey = itdsl.Given(t).Tenant(itdsl.TenantOpts{Alias: "other", Isolated: true}).Build().
		Tenant("other").OwnerClient
	return w
}

func createCustomRole(
	t *testing.T, product *itdsl.ProductContext, organizationID, resourceType string,
) ct.ProductRoleResponse {
	t.Helper()
	resp, err := product.AllScopeAPIKeyClient().CreateOrganizationRoleWithResponse(
		context.Background(), product.ProductID, organizationID,
		ct.CreateOrganizationRoleJSONRequestBody{Name: "Custom " + ids.MustNew("ct"), ResourceType: resourceType},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return *resp.JSON201
}

// The folder of Acme, driven by another Organization's path.
func (w idorWorld) folderViaOtherOrg() itdsl.ResourceMemberClient {
	return w.product.ResourceMembers(w.otherOrg.Id, "folder", w.folder)
}

// The folder of Acme, driven by a credential of another Product.
func (w idorWorld) folderAs(client *ct.ClientWithResponses) itdsl.ResourceMemberClient {
	return w.folderMembers().As(client)
}

// requireFolderUntouched proves a refused write left Acme's folder and Dan's
// role on it as they were.
func (w idorWorld) requireFolderUntouched(t *testing.T) {
	t.Helper()
	assert.Equal(t, "Design", w.resources.Get("folder", w.folder).Name)
	assert.Equal(t, []string{w.granter.Id}, roleIDs(w.folderMembers().Get(w.member)))
}

func TestResourcesRefuseAnotherOrganization(t *testing.T) {
	t.Parallel()
	w := givenIDORWorld(t)
	viaOther := w.product.Resources(w.otherOrg.Id)

	t.Run("Reading Acme's folder through another Organization answers 404", func(t *testing.T) {
		resp := viaOther.GetRaw("folder", w.folder)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})

	t.Run("Deleting Acme's folder through another Organization answers 404 and keeps it", func(t *testing.T) {
		resp := viaOther.DeleteRaw("folder", w.folder, true)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
		w.requireFolderUntouched(t)
	})

	t.Run("Searching another Organization never lists Acme's resources", func(t *testing.T) {
		found := viaOther.Search(ct.SearchResourcesJSONRequestBody{})

		assert.Empty(t, found.Items)
	})

	t.Run("Assigning a role on Acme's folder through another Organization answers 404", func(t *testing.T) {
		resp := w.folderViaOtherOrg().PutRaw(w.otherMember, w.editor.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
		w.requireFolderUntouched(t)
	})

	t.Run("Reading Dan's role on Acme's folder through another Organization answers 404", func(t *testing.T) {
		resp := w.folderViaOtherOrg().GetRaw(w.member)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})

	t.Run("Removing Dan's role through another Organization answers 404 and keeps it", func(t *testing.T) {
		resp := w.folderViaOtherOrg().DeleteRaw(w.member)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
		w.requireFolderUntouched(t)
	})

	t.Run("Listing the members of Acme's folder through another Organization answers 404", func(t *testing.T) {
		resp := w.folderViaOtherOrg().SearchRaw(ct.SearchResourceMembersJSONRequestBody{})

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "RESOURCE_NOT_FOUND")
	})

	t.Run("A member of another Organization can't get a role on Acme's folder", func(t *testing.T) {
		resp := w.folderMembers().PutRaw(w.otherMember, w.editor.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "ORGANIZATION_MEMBERSHIP_NOT_FOUND")
	})

	t.Run("Another Organization's custom role can't be held on Acme's folder", func(t *testing.T) {
		resp := w.folderMembers().PutRaw(w.member, w.otherCustom.Id)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_NOT_FOUND_IN_REQUEST")
		w.requireFolderUntouched(t)
	})

	t.Run("Acme's custom role can't be read or deleted through another Organization", func(t *testing.T) {
		client := w.product.AllScopeAPIKeyClient()
		read, err := client.GetOrganizationRoleWithResponse(
			context.Background(), w.product.ProductID, w.otherOrg.Id, w.ownCustom.Id)
		require.NoError(t, err)
		deleted, err := client.DeleteOrganizationRoleWithResponse(
			context.Background(), w.product.ProductID, w.otherOrg.Id, w.ownCustom.Id)
		require.NoError(t, err)

		requireAPIError(t, read, read.Body, http.StatusNotFound, "ROLE_NOT_FOUND")
		requireAPIError(t, deleted, deleted.Body, http.StatusNotFound, "ROLE_NOT_FOUND")
		kept, err := client.GetOrganizationRoleWithResponse(
			context.Background(), w.product.ProductID, w.org.Id, w.ownCustom.Id)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, kept.StatusCode())
	})

	t.Run("A member of another Organization is never allowed on Acme's folder, while Dan is", func(t *testing.T) {
		externalID := "user_" + ids.MustNew("ct")
		setDSLProductUserExternalID(t, w.product, w.otherMember, externalID)
		access := w.product.Access()

		dan := access.User(w.danID).OnResource(w.org.Id, "folder", w.folder).Can("folder:edit").Decide()
		inAcme := access.User(externalID).OnResource(w.org.Id, "folder", w.folder).Can("folder:edit").Decide()
		inOwnOrg := access.User(externalID).OnResource(w.otherOrg.Id, "folder", w.folder).Can("folder:edit").Decide()

		assert.True(t, dan.Decision)
		assert.False(t, inAcme.Decision)
		assert.False(t, inOwnOrg.Decision)
	})
}

func TestResourcesRefuseAnotherProduct(t *testing.T) {
	t.Parallel()
	w := givenIDORWorld(t)

	t.Run("Another Product's key can't read, write or delete Acme's folder", func(t *testing.T) {
		resources := w.resources.As(w.otherKey)

		read := resources.GetRaw("folder", w.folder)
		written := resources.PutRaw("folder", w.folder, "Taken", itdsl.Ref("project", w.project))
		deleted := resources.DeleteRaw("folder", w.folder, true)
		searched := resources.SearchRaw(ct.SearchResourcesJSONRequestBody{})

		requireAPIError(t, read, read.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		requireAPIError(t, written, written.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		requireAPIError(t, deleted, deleted.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		requireAPIError(t, searched, searched.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		w.requireFolderUntouched(t)
	})

	t.Run("Another Product's key can't read, assign or remove roles on Acme's folder", func(t *testing.T) {
		members := w.folderAs(w.otherKey)

		read := members.GetRaw(w.member)
		assigned := members.PutRaw(w.member, w.viewer.Id)
		removed := members.DeleteRaw(w.member)
		listed := members.SearchRaw(ct.SearchResourceMembersJSONRequestBody{})

		requireAPIError(t, read, read.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		requireAPIError(t, assigned, assigned.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		requireAPIError(t, removed, removed.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		requireAPIError(t, listed, listed.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		w.requireFolderUntouched(t)
	})

	t.Run("Another Product's role can't be held on Acme's folder", func(t *testing.T) {
		resp := w.folderMembers().PutRaw(w.member, w.otherRole.Id)

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_NOT_FOUND_IN_REQUEST")
		w.requireFolderUntouched(t)
	})

	t.Run("Another Product's user can't get a role on Acme's folder", func(t *testing.T) {
		resp := w.folderMembers().PutRaw(w.otherUser, w.editor.Id)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "ORGANIZATION_MEMBERSHIP_NOT_FOUND")
	})

	t.Run("Another Product's key can't evaluate access on Acme's product", func(t *testing.T) {
		resp := w.product.AccessClientWith(w.otherKey).
			User(w.danID).
			OnResource(w.org.Id, "folder", w.folder).
			Can("folder:edit").
			Send()

		requireAPIError(t, resp, resp.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
	})

	t.Run("Another Product's key can't change Acme's resource types", func(t *testing.T) {
		types := w.product.ResourceTypes()
		folder := types.BySlug("folder")
		off := false

		resp, err := w.otherKey.UpdateProductResourceTypeWithResponse(context.Background(), w.product.ProductID,
			folder.Id, ct.UpdateProductResourceTypeJSONRequestBody{AllowsCustomRoles: &off})
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
		assert.True(t, types.Get(folder.Id).AllowsCustomRoles)
	})

	t.Run("Another Product's key can't read Acme's custom role", func(t *testing.T) {
		resp, err := w.otherKey.GetOrganizationRoleWithResponse(
			context.Background(), w.product.ProductID, w.org.Id, w.ownCustom.Id)
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusUnauthorized, "INVALID_PRODUCT_API_KEY")
	})
}

func TestResourcesRefuseAnotherTenant(t *testing.T) {
	t.Parallel()
	w := givenIDORWorld(t)

	t.Run("The owner's platform session reads Acme's folder and Dan's role on it", func(t *testing.T) {
		read := w.resources.As(w.product.OwnerAuthenticatedClient()).GetRaw("folder", w.folder)
		roles := w.folderAs(w.product.OwnerAuthenticatedClient()).GetRaw(w.member)

		assert.Equal(t, http.StatusOK, read.StatusCode(), string(read.Body))
		assert.Equal(t, http.StatusOK, roles.StatusCode(), string(roles.Body))
	})

	t.Run("Another tenant's platform session can't read Acme's folder or its roles", func(t *testing.T) {
		read := w.resources.As(w.otherTenantKey).GetRaw("folder", w.folder)
		roles := w.folderAs(w.otherTenantKey).GetRaw(w.member)
		listed := w.resources.As(w.otherTenantKey).SearchRaw(ct.SearchResourcesJSONRequestBody{})

		requireAPIError(t, read, read.Body, http.StatusNotFound, "PRODUCT_NOT_FOUND")
		requireAPIError(t, roles, roles.Body, http.StatusNotFound, "PRODUCT_NOT_FOUND")
		requireAPIError(t, listed, listed.Body, http.StatusNotFound, "PRODUCT_NOT_FOUND")
	})

	t.Run("A platform session never writes a role on a resource, own tenant or not", func(t *testing.T) {
		own := w.folderAs(w.product.OwnerAuthenticatedClient()).PutRaw(w.member, w.viewer.Id)
		foreign := w.folderAs(w.otherTenantKey).PutRaw(w.member, w.viewer.Id)

		requireAPIError(t, own, own.Body, http.StatusUnauthorized, "UNAUTHORIZED")
		requireAPIError(t, foreign, foreign.Body, http.StatusUnauthorized, "UNAUTHORIZED")
		w.requireFolderUntouched(t)
	})

	t.Run("Another tenant's platform session can't read Acme's custom role", func(t *testing.T) {
		resp, err := w.otherTenantKey.GetOrganizationRoleWithResponse(
			context.Background(), w.product.ProductID, w.org.Id, w.ownCustom.Id)
		require.NoError(t, err)

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "PRODUCT_NOT_FOUND")
	})
}
