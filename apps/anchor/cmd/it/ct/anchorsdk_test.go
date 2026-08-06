package ct_test

import (
	"context"
	"net/http"
	"testing"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

// TestAnchorSDK drives clients/go/anchorsdk against the running server, so the
// SDK anchor ships is exercised by anchor's own CI rather than only by its
// consumers. The other CT files assert the wire contract — status codes and
// typed error bodies — which the SDK deliberately hides; this one asserts the
// behaviour a product backend actually programs against.
func TestAnchorSDK(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	sdk := product.SDKClient()

	require.Equal(t, product.ProductID, sdk.ProductID(), "the product id is bound at construction")

	t.Run("Organizations", func(t *testing.T) {
		// Metadata is set here on purpose even though nothing asserts it back:
		// the schema carries it on both the request and the response, but the
		// server drops it — CreateOrganizationInput has no metadata field and
		// mapOrganizationToResponse never populates one. Sending it keeps this
		// test honest about what the SDK puts on the wire, and this assertion
		// gets its round-trip check once the server implements it.
		created, err := sdk.Organizations().
			Create("SDK Org "+itshared.Faker.UUID().V4()).
			Description("created through anchorsdk").
			Meta("billing_ref", "cust_sdk").
			Metadata(map[string]any{"region": "us-east-1"}).
			Do(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, created.Id)
		require.NotNil(t, created.Description)
		assert.Equal(t, "created through anchorsdk", *created.Description)

		fetched, err := sdk.Organizations().Get(ctx, created.Id)
		require.NoError(t, err)
		assert.Equal(t, created.Id, fetched.Id)
		assert.Equal(t, created.Name, fetched.Name)

		updated, err := sdk.Organizations().
			Update(created.Id, created.Name+" (renamed)").
			Description("updated through anchorsdk").
			Do(ctx)
		require.NoError(t, err)
		assert.Equal(t, created.Name+" (renamed)", updated.Name)

		page, err := sdk.Organizations().Search().IDs(created.Id).Limit(10).Do(ctx)
		require.NoError(t, err)
		require.Len(t, page.Items, 1, "the id filter reaches the server")
		assert.Equal(t, created.Id, page.Items[0].Id)

		require.NoError(t, sdk.Organizations().Delete(ctx, created.Id))

		_, err = sdk.Organizations().Get(ctx, created.Id)
		require.Error(t, err)
		assert.ErrorIs(t, err, anchorsdk.ErrNotFound, "a deleted organization is gone")
	})

	t.Run("Members", func(t *testing.T) {
		role := createDSLProductRole(t, product, "SDK Member Role", nil)
		productUser := createDSLProductUser(t, product)

		org, err := sdk.Organizations().Create("SDK Members " + itshared.Faker.UUID().V4()).Do(ctx)
		require.NoError(t, err)
		handle := sdk.Organization(org.Id)
		require.Equal(t, org.Id, handle.ID())

		added, err := handle.Members().Add(ctx, productUser.ID, role.ID)
		require.NoError(t, err)
		assert.Equal(t, productUser.ID, added.ProductUserId)

		listed, err := handle.Members().List(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, listed.Items)

		got, err := handle.Members().Get(ctx, productUser.ID)
		require.NoError(t, err)
		assert.Equal(t, productUser.ID, got.ProductUserId)

		withPerms, err := handle.Members().GetWithRolePermissions(ctx, productUser.ID)
		require.NoError(t, err)
		assert.Equal(t, productUser.ID, withPerms.ProductUserId)

		otherRole := createDSLProductRole(t, product, "SDK Member Role 2", nil)
		reroled, err := handle.Members().SetRole(ctx, productUser.ID, otherRole.ID)
		require.NoError(t, err)
		assert.Equal(t, otherRole.ID, reroled.Role.Id)

		require.NoError(t, handle.Members().Remove(ctx, productUser.ID))

		_, err = handle.Members().Get(ctx, productUser.ID)
		assert.ErrorIs(t, err, anchorsdk.ErrNotFound, "a removed member is gone")
	})

	t.Run("Workspaces", func(t *testing.T) {
		org, err := sdk.Organizations().Create("SDK Workspaces " + itshared.Faker.UUID().V4()).Do(ctx)
		require.NoError(t, err)
		handle := sdk.Organization(org.Id)

		created, err := handle.Workspaces().
			Create("Production").
			Description("created through anchorsdk").
			Do(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, created.Id)

		fetched, err := handle.Workspaces().Get(ctx, created.Id)
		require.NoError(t, err)
		assert.Equal(t, created.Id, fetched.Id)

		updated, err := handle.Workspaces().Update(created.Id, "Staging").Do(ctx)
		require.NoError(t, err)
		assert.Equal(t, "Staging", updated.Name)

		page, err := handle.Workspaces().List(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, page.Items)

		require.NoError(t, handle.Workspaces().Delete(ctx, created.Id))
	})

	t.Run("APIKeys", func(t *testing.T) {
		permissions := product.CreateDefaultProductResourcePermissions(t)
		require.NotEmpty(t, permissions)
		scope := permissions[0].Name

		org, err := sdk.Organizations().Create("SDK API Keys " + itshared.Faker.UUID().V4()).Do(ctx)
		require.NoError(t, err)
		handle := sdk.Organization(org.Id)

		created, err := handle.APIKeys().
			Create("sdk-ci").
			Description("created through anchorsdk").
			Permissions(scope).
			Do(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, created.Value, "the clear key is returned exactly once, at creation")

		fetched, err := handle.APIKeys().Get(ctx, created.Id)
		require.NoError(t, err)
		assert.Equal(t, created.Id, fetched.Id)
		assert.NotEqual(t, created.Value, fetched.ObfuscatedValue, "later reads obfuscate the value")

		updated, err := handle.APIKeys().Update(created.Id, "sdk-ci-renamed").Do(ctx)
		require.NoError(t, err)
		assert.Equal(t, "sdk-ci-renamed", updated.Name)

		page, err := handle.APIKeys().Search().IDs(created.Id).Do(ctx)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)

		validated, err := handle.APIKeys().Validate(ctx, created.Value, scope)
		require.NoError(t, err)
		assert.Equal(t, created.Id, validated.ApiKey.Id)
		assert.Empty(t, validated.MissingPrivileges)

		require.NoError(t, handle.APIKeys().Delete(ctx, created.Id))
	})

	t.Run("Introspect", func(t *testing.T) {
		permissions := product.CreateProductResourcePermissions(t,
			"sdk:read:"+itshared.Faker.UUID().V4(),
			"sdk:write:"+itshared.Faker.UUID().V4(),
		)
		require.Len(t, permissions, 2)
		granted, withheld := permissions[0].Name, permissions[1].Name

		org, err := sdk.Organizations().Create("SDK Introspect " + itshared.Faker.UUID().V4()).Do(ctx)
		require.NoError(t, err)

		key, err := sdk.Organization(org.Id).APIKeys().
			Create("sdk-introspect").
			Permissions(granted).
			Do(ctx)
		require.NoError(t, err)

		t.Run("resolves the organization without being told it", func(t *testing.T) {
			identity, introErr := sdk.Introspect(ctx, key.Value)
			require.NoError(t, introErr)
			assert.Equal(t, key.Id, identity.ApiKey.Id)
			assert.Equal(t, org.Id, identity.ApiKey.OrganizationId)
			assert.ElementsMatch(t, []string{granted}, identity.Permissions)
		})

		t.Run("satisfied scopes succeed", func(t *testing.T) {
			identity, introErr := sdk.Introspect(ctx, key.Value, granted)
			require.NoError(t, introErr)
			assert.Empty(t, identity.MissingPrivileges)
		})

		t.Run("a missing scope is forbidden and permanent", func(t *testing.T) {
			_, introErr := sdk.Introspect(ctx, key.Value, withheld)
			require.Error(t, introErr)
			require.ErrorIs(t, introErr, anchorsdk.ErrForbidden)
			require.ErrorIs(t, introErr, anchorsdk.ErrPermanent, "retrying cannot grant a scope")

			var apiErr *anchorsdk.Error
			require.ErrorAs(t, introErr, &apiErr)
			assert.Contains(t, string(apiErr.Body), withheld,
				"the body naming the missing scopes stays reachable on Error.Body")
		})

		t.Run("an unknown key is not found", func(t *testing.T) {
			_, introErr := sdk.Introspect(ctx, "anchor_org_apikey_not_a_real_key")
			require.Error(t, introErr)
			assert.ErrorIs(t, introErr, anchorsdk.ErrNotFound)
		})
	})

	t.Run("Users", func(t *testing.T) {
		email := itshared.Faker.Internet().Email()

		created, err := sdk.Users().Create(email).
			Name("SDK User").
			Status(nanoclient.Active).
			Do(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, created.Id)
		assert.Equal(t, email, created.Email)

		fetched, err := sdk.Users().Get(ctx, created.Id)
		require.NoError(t, err)
		assert.Equal(t, created.Id, fetched.Id)

		page, err := sdk.Users().Search().Emails(email).Do(ctx)
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		assert.Equal(t, created.Id, page.Items[0].Id)

		t.Run("organizations reflect membership", func(t *testing.T) {
			role := createDSLProductRole(t, product, "SDK User Role", nil)
			org, orgErr := sdk.Organizations().Create("SDK User Orgs " + itshared.Faker.UUID().V4()).Do(ctx)
			require.NoError(t, orgErr)

			before, listErr := sdk.Users().Organizations(ctx, created.Id)
			require.NoError(t, listErr)
			assert.Empty(t, before.Items, "a fresh user belongs to nothing")

			_, addErr := sdk.Organization(org.Id).Members().Add(ctx, created.Id, role.ID)
			require.NoError(t, addErr)

			after, listErr := sdk.Users().OrganizationsWithRolePermissions(ctx, created.Id)
			require.NoError(t, listErr)
			require.Len(t, after.Items, 1)
			assert.Equal(t, org.Id, after.Items[0].Organization.Id)

			one, getErr := sdk.Users().Organization(ctx, created.Id, org.Id)
			require.NoError(t, getErr)
			assert.Equal(t, org.Id, one.Organization.Id)
		})

		require.NoError(t, sdk.Users().Delete(ctx, created.Id))

		_, err = sdk.Users().Get(ctx, created.Id)
		assert.ErrorIs(t, err, anchorsdk.ErrNotFound)
	})

	t.Run("ErrorClassification", func(t *testing.T) {
		_, err := sdk.Organizations().Get(ctx, ids.MustNew("org"))
		require.Error(t, err)

		require.ErrorIs(t, err, anchorsdk.ErrNotFound)
		require.ErrorIs(t, err, anchorsdk.ErrPermanent)
		require.NotErrorIs(t, err, anchorsdk.ErrConflict)

		var apiErr *anchorsdk.Error
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, "Organizations.Get", apiErr.Op)
		assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
	})

	t.Run("RawReachesOperationsOutsideTheSDKScope", func(t *testing.T) {
		// Product roles are admin surface and deliberately unwrapped; Raw is the
		// documented escape hatch for them.
		resp, err := sdk.Raw().SearchProductRolesWithResponse(
			ctx,
			product.ProductID,
			nanoclient.SearchProductRolesJSONRequestBody{},
		)
		require.NoError(t, err)
		require.NotNil(t, resp.JSON200)
	})
}
