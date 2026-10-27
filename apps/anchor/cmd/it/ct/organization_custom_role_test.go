package ct_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// customRoleWorld is a product whose organization type allows custom roles,
// with Acme and Globex and one managed role.
type customRoleWorld struct {
	accessWorld
	client  *ct.ClientWithResponses
	orgType ct.ProductResourceTypeResponse
}

func newCustomRoleWorld(t *testing.T) customRoleWorld {
	t.Helper()
	w := customRoleWorld{accessWorld: newAccessWorld(t)}
	w.client = w.product.OwnerAuthenticatedClient()
	w.orgType = w.product.ResourceTypes().BySlug("organization")
	w.allowCustomRoles(t, true)
	return w
}

func (w customRoleWorld) allowCustomRoles(t *testing.T, allowed bool) {
	t.Helper()
	updated := w.product.ResourceTypes().Update(w.orgType.Id, ct.UpdateProductResourceTypeJSONRequestBody{
		AllowsCustomRoles: new(allowed),
	})
	require.Equal(t, allowed, updated.AllowsCustomRoles)
}

func (w customRoleWorld) createRaw(
	t *testing.T, organizationID, name string, permissions ...string,
) *ct.CreateOrganizationRoleResponse {
	t.Helper()
	resp, err := w.client.CreateOrganizationRoleWithResponse(context.Background(), w.product.ProductID, organizationID,
		ct.CreateOrganizationRoleJSONRequestBody{Name: name, ResourceType: "organization", Permissions: permissions})
	require.NoError(t, err)
	return resp
}

func (w customRoleWorld) create(
	t *testing.T, organizationID, name string, permissions ...string,
) ct.ProductRoleResponse {
	t.Helper()
	resp := w.createRaw(t, organizationID, name, permissions...)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	return *resp.JSON201
}

func (w customRoleWorld) search(
	t *testing.T, organizationID string, filter *ct.OrganizationRoleFilter,
) []ct.ProductRoleResponse {
	t.Helper()
	resp, err := w.client.SearchOrganizationRolesWithResponse(context.Background(), w.product.ProductID,
		organizationID, ct.SearchOrganizationRolesJSONRequestBody{Filter: filter})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	return resp.JSON200.Items
}

func (w customRoleWorld) updateRaw(
	t *testing.T, organizationID, roleID string, body ct.UpdateOrganizationRoleJSONRequestBody,
) *ct.UpdateOrganizationRoleResponse {
	t.Helper()
	resp, err := w.client.UpdateOrganizationRoleWithResponse(
		context.Background(), w.product.ProductID, organizationID, roleID, body)
	require.NoError(t, err)
	return resp
}

func (w customRoleWorld) deleteRaw(t *testing.T, organizationID, roleID string) *ct.DeleteOrganizationRoleResponse {
	t.Helper()
	resp, err := w.client.DeleteOrganizationRoleWithResponse(
		context.Background(), w.product.ProductID, organizationID, roleID)
	require.NoError(t, err)
	return resp
}

func roleNames(roles []ct.ProductRoleResponse) []string {
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.Name)
	}
	return names
}

func TestCustomRoleSwitch(t *testing.T) {
	t.Parallel()

	t.Run("A product starts with custom roles off", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		org := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
		w := customRoleWorld{product: product, client: product.OwnerAuthenticatedClient()}

		resp := w.createRaw(t, org.Id, "Billing auditor")

		assert.False(t, product.ResourceTypes().BySlug("organization").AllowsCustomRoles)
		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
	})

	t.Run("The organization type turns custom roles on", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)

		assert.True(t, w.product.ResourceTypes().Get(w.orgType.Id).AllowsCustomRoles)
	})

	t.Run("The organization type still refuses any other change", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)

		resp := w.product.ResourceTypes().UpdateRaw(w.orgType.Id, ct.UpdateProductResourceTypeJSONRequestBody{
			Name: new("Workspace"), AllowsCustomRoles: new(true),
		})

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "BUILT_IN_RESOURCE_TYPE")
	})

	t.Run("A type of the product's own refuses custom roles", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		project := product.ResourceTypes().CreateUnder("project", "organization")

		resp := product.ResourceTypes().UpdateRaw(project.Id, ct.UpdateProductResourceTypeJSONRequestBody{
			AllowsCustomRoles: new(true),
		})

		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "CUSTOM_ROLES_NOT_AVAILABLE")
	})

	t.Run("Turning the switch off keeps a custom role and the access it grants", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor", w.read)
		_, externalID := w.member(t, w.orgA.Id, custom.Id)

		w.allowCustomRoles(t, false)
		decision := w.access.User(externalID).InOrganization(w.orgA.Id).Can(w.read).Decide()

		assert.Contains(t, roleNames(w.search(t, w.orgA.Id, nil)), "Billing auditor")
		assert.True(t, decision.Decision)
	})

	t.Run("With the switch off a custom role can't be created or changed", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor")
		w.allowCustomRoles(t, false)

		created := w.createRaw(t, w.orgA.Id, "Auditor")
		updated := w.updateRaw(t, w.orgA.Id, custom.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: "Auditor"})

		requireAPIError(t, created, created.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
		requireAPIError(t, updated, updated.Body, http.StatusBadRequest, "CUSTOM_ROLES_DISABLED")
	})

	t.Run("With the switch off a custom role can still be deleted", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor")
		w.allowCustomRoles(t, false)

		resp := w.deleteRaw(t, w.orgA.Id, custom.Id)

		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
	})
}

func TestOrganizationCustomRoles(t *testing.T) {
	t.Parallel()

	t.Run("A created custom role belongs to its organization", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		sink := w.product.CaptureEvents()

		custom := w.create(t, w.orgA.Id, "Billing auditor", w.read)

		assert.Equal(t, ct.CUSTOM, custom.Type)
		require.NotNil(t, custom.OrganizationId)
		assert.Equal(t, w.orgA.Id, *custom.OrganizationId)
		assert.Equal(t, "organization", custom.ResourceType)
		require.Len(t, custom.Permissions, 1)
		sink.WaitFor("organization.role.created", map[string]string{"role_id": custom.Id, "organization_id": w.orgA.Id})
	})

	t.Run("An organization lists the managed roles and its own custom roles", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		w.create(t, w.orgA.Id, "Billing auditor")

		inAcme := roleNames(w.search(t, w.orgA.Id, nil))
		inGlobex := roleNames(w.search(t, w.orgB.Id, nil))

		assert.ElementsMatch(t, []string{w.role.Name, "Billing auditor"}, inAcme)
		assert.ElementsMatch(t, []string{w.role.Name}, inGlobex)
	})

	t.Run("A search filters by role type", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		w.create(t, w.orgA.Id, "Billing auditor")

		custom := w.search(t, w.orgA.Id, &ct.OrganizationRoleFilter{Types: []ct.ProductRoleType{ct.CUSTOM}})
		managed := w.search(t, w.orgA.Id, &ct.OrganizationRoleFilter{Types: []ct.ProductRoleType{ct.MANAGED}})

		assert.Equal(t, []string{"Billing auditor"}, roleNames(custom))
		assert.Equal(t, []string{w.role.Name}, roleNames(managed))
	})

	t.Run("The product's role routes leave custom roles out", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor")

		resp, err := w.client.GetProductRoleWithResponse(context.Background(), w.product.ProductID, custom.Id)

		require.NoError(t, err)
		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "ROLE_NOT_FOUND")
	})

	t.Run("Another organization can't read a custom role", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor")

		inAcme, err := w.client.GetOrganizationRoleWithResponse(
			context.Background(), w.product.ProductID, w.orgA.Id, custom.Id)
		require.NoError(t, err)
		inGlobex, err := w.client.GetOrganizationRoleWithResponse(
			context.Background(), w.product.ProductID, w.orgB.Id, custom.Id)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, inAcme.StatusCode(), string(inAcme.Body))
		requireAPIError(t, inGlobex, inGlobex.Body, http.StatusNotFound, "ROLE_NOT_FOUND")
	})

	t.Run("An unknown organization answers not found", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)

		resp, err := w.client.SearchOrganizationRolesWithResponse(context.Background(), w.product.ProductID,
			ids.MustNew("org"), ct.SearchOrganizationRolesJSONRequestBody{})

		require.NoError(t, err)
		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "ORGANIZATION_NOT_FOUND")
	})

	t.Run("A member holds a custom role and gets its permissions", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor", w.read)
		_, externalID := w.member(t, w.orgA.Id, custom.Id)

		decision := w.access.User(externalID).InOrganization(w.orgA.Id).Can(w.read).Decide()

		assert.True(t, decision.Decision)
	})

	t.Run("A member of another organization can't hold a custom role", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor")
		user := createDSLProductUser(t, w.product)

		resp, err := w.client.AddOrganizationMemberWithResponse(context.Background(), w.product.ProductID, w.orgB.Id,
			ct.AddOrganizationMemberJSONRequestBody{ProductUserId: user.ID, RoleId: new(custom.Id)})

		require.NoError(t, err)
		requireAPIError(t, resp, resp.Body, http.StatusBadRequest, "ROLE_NOT_FOUND_IN_REQUEST")
	})
}

func TestOrganizationCustomRoleNames(t *testing.T) {
	t.Parallel()

	t.Run("A custom role can't take a managed role's name", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)

		resp := w.createRaw(t, w.orgA.Id, w.role.Name)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_NAME_DUPLICATE")
	})

	t.Run("A custom role name is unique in its organization, whatever the case", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		w.create(t, w.orgA.Id, "Billing auditor")

		resp := w.createRaw(t, w.orgA.Id, "BILLING AUDITOR")

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_NAME_DUPLICATE")
	})

	t.Run("Two organizations may use the same custom role name", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		w.create(t, w.orgA.Id, "Billing auditor")

		resp := w.createRaw(t, w.orgB.Id, "Billing auditor")

		assert.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	})

	t.Run("A managed role may take a custom role's name", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		w.create(t, w.orgA.Id, "Billing auditor")

		resp, err := w.client.CreateProductRoleWithResponse(context.Background(), w.product.ProductID,
			ct.CreateProductRoleJSONRequestBody{Name: "Billing auditor", ResourceType: "organization"})

		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	})
}

func TestOrganizationCustomRoleLimit(t *testing.T) {
	t.Parallel()

	t.Run("An organization holds at most 50 custom roles", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		for i := range 50 {
			w.create(t, w.orgA.Id, fmt.Sprintf("Role %02d", i))
		}

		resp := w.createRaw(t, w.orgA.Id, "Role 50")
		other := w.createRaw(t, w.orgB.Id, "Role 50")

		meta := requireAPIError(t, resp, resp.Body, http.StatusConflict, "CUSTOM_ROLE_LIMIT_REACHED")
		assert.InDelta(t, 50, meta["max_custom_roles"], 0)
		assert.Equal(t, http.StatusCreated, other.StatusCode(), "the limit counts one organization")
	})

	t.Run("Two creates at 49 custom roles never pass the limit", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		for i := range 49 {
			w.create(t, w.orgA.Id, fmt.Sprintf("Role %02d", i))
		}

		statuses := make([]int, 2)
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := range statuses {
			wg.Go(func() {
				<-start
				resp, err := w.client.CreateOrganizationRoleWithResponse(context.Background(), w.product.ProductID,
					w.orgA.Id, ct.CreateOrganizationRoleJSONRequestBody{
						Name: fmt.Sprintf("Racer %d", i), ResourceType: "organization",
					})
				if assert.NoError(t, err) {
					statuses[i] = resp.StatusCode()
				}
			})
		}
		close(start)
		wg.Wait()

		assert.ElementsMatch(t, []int{http.StatusCreated, http.StatusConflict}, statuses)
		resp, err := w.client.SearchOrganizationRolesWithResponse(context.Background(), w.product.ProductID,
			w.orgA.Id, ct.SearchOrganizationRolesJSONRequestBody{
				Filter: &ct.OrganizationRoleFilter{Types: []ct.ProductRoleType{ct.CUSTOM}},
			})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.EqualValues(t, 50, resp.JSON200.Total)
	})
}

func TestOrganizationCustomRoleChanges(t *testing.T) {
	t.Parallel()

	t.Run("An organization changes its custom role", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor")
		sink := w.product.CaptureEvents()

		resp := w.updateRaw(t, w.orgA.Id, custom.Id, ct.UpdateOrganizationRoleJSONRequestBody{
			Name: "Auditor", Permissions: &[]string{w.read},
		})

		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))
		assert.Equal(t, "Auditor", resp.JSON200.Name)
		require.Len(t, resp.JSON200.Permissions, 1)
		assert.Equal(t, w.read, resp.JSON200.Permissions[0].PermissionName)
		sink.WaitFor("organization.role.updated", map[string]string{"role_id": custom.Id, "organization_id": w.orgA.Id})
	})

	t.Run("An organization can't change or delete a managed role", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)

		updated := w.updateRaw(t, w.orgA.Id, w.role.ID, ct.UpdateOrganizationRoleJSONRequestBody{Name: "Mine"})
		deleted := w.deleteRaw(t, w.orgA.Id, w.role.ID)

		requireAPIError(t, updated, updated.Body, http.StatusConflict, "ROLE_NOT_EDITABLE")
		requireAPIError(t, deleted, deleted.Body, http.StatusConflict, "ROLE_NOT_EDITABLE")
	})

	t.Run("Another organization can't change a custom role", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor")

		resp := w.updateRaw(t, w.orgB.Id, custom.Id, ct.UpdateOrganizationRoleJSONRequestBody{Name: "Mine"})

		requireAPIError(t, resp, resp.Body, http.StatusNotFound, "ROLE_NOT_FOUND")
	})

	t.Run("A custom role a member holds can't be deleted", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor")
		w.member(t, w.orgA.Id, custom.Id)

		resp := w.deleteRaw(t, w.orgA.Id, custom.Id)

		requireAPIError(t, resp, resp.Body, http.StatusConflict, "ROLE_IN_USE")
	})

	t.Run("A custom role nobody holds is deleted", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor")
		sink := w.product.CaptureEvents()

		resp := w.deleteRaw(t, w.orgA.Id, custom.Id)

		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		assert.NotContains(t, roleNames(w.search(t, w.orgA.Id, nil)), "Billing auditor")
		sink.WaitFor("organization.role.deleted", map[string]string{"role_id": custom.Id, "organization_id": w.orgA.Id})
	})

	t.Run("A permission delete removes it from custom roles too", func(t *testing.T) {
		t.Parallel()
		w := newCustomRoleWorld(t)
		custom := w.create(t, w.orgA.Id, "Billing auditor", w.read)
		sink := w.product.CaptureEvents()

		resp, err := w.client.DeleteProductResourcePermissionWithResponse(
			context.Background(), w.product.ProductID, w.read)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode(), string(resp.Body))
		found, err := w.client.GetOrganizationRoleWithResponse(
			context.Background(), w.product.ProductID, w.orgA.Id, custom.Id)

		require.NoError(t, err)
		assert.Empty(t, found.JSON200.Permissions)
		sink.WaitFor("organization.role.updated", map[string]string{"role_id": custom.Id, "organization_id": w.orgA.Id})
	})
}
