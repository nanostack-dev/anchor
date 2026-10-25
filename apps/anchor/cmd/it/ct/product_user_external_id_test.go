package ct_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const productUserExternalIDMaxLength = 255

func newExternalID() string {
	return "usr_" + uuid.NewString()
}

func newProductUserBody(externalID *string) ct.CreateProductUserJSONRequestBody {
	name := itshared.Faker.Person().Name()
	return ct.CreateProductUserJSONRequestBody{
		Email:      itshared.Faker.Internet().Email(),
		Name:       &name,
		ExternalId: externalID,
	}
}

func requireExternalIDRefused(t *testing.T, resp *ct.CreateProductUserResponse, rule string) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	require.NotNil(t, resp.JSON400)
	require.Len(t, resp.JSON400.Errors, 1)
	refusal := resp.JSON400.Errors[0]
	assert.Equal(t, "VALIDATION_ERROR", refusal.Code)
	require.NotNil(t, refusal.Metadata)
	assert.Equal(t, "ExternalID", (*refusal.Metadata)["field"])
	assert.Equal(t, rule, (*refusal.Metadata)["rule"])
}

// readerMembership gives a fresh organization and a role holding one permission.
func readerMembership(t *testing.T, product *itdsl.ProductContext) (string, *itdsl.ProductRole, string) {
	t.Helper()
	permission := givenOrganizationAPIKeyResourcePermissions(t, product).FileRead
	role := createDSLProductRoleWithPermissions(t, product, "reader-"+uuid.NewString(), nil, []string{permission})
	organization := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
	return organization.Id, role, permission
}

func TestCreateProductUserWithAnExternalID(t *testing.T) {
	t.Parallel()
	t.Run(
		"A product user created with an external ID is allowed by access evaluation in its organization",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			organizationID, role, permission := readerMembership(t, product)
			externalID := newExternalID()

			created := product.ProductUsers().Create(newProductUserBody(&externalID))
			require.NotNil(t, created.ExternalId)
			assert.Equal(t, externalID, *created.ExternalId)
			fetched := product.ProductUsers().Get(created.Id)
			require.NotNil(t, fetched.ExternalId)
			assert.Equal(t, externalID, *fetched.ExternalId)

			createDSLMembership(t, product, created.Id, organizationID, role.ID)
			response := product.Access().User(externalID).InOrganization(organizationID).Can(permission).Decide()

			assert.True(t, response.Decision)
			assert.Nil(t, response.Context.Reason)
			require.NotNil(t, response.Context.ProductUser)
			assert.Equal(t, created.Id, response.Context.ProductUser.Id)
			require.NotNil(t, response.Context.Role)
			assert.Equal(t, role.ID, response.Context.Role.Id)
		},
	)
}

func TestCreateProductUserWithoutAnExternalID(t *testing.T) {
	t.Parallel()
	t.Run("A product user created without an external ID has none", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		created := product.ProductUsers().Create(newProductUserBody(nil))

		assert.Nil(t, created.ExternalId)
		assert.Nil(t, product.ProductUsers().Get(created.Id).ExternalId)
	})
}

func TestCreateProductUserWithATakenExternalID(t *testing.T) {
	t.Parallel()
	t.Run("A second product user with the same external ID is refused with a conflict", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		externalID := newExternalID()
		product.ProductUsers().Create(newProductUserBody(&externalID))

		resp := product.ProductUsers().CreateRaw(newProductUserBody(&externalID))

		require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
		require.NotNil(t, resp.JSON409)
		require.Len(t, resp.JSON409.Errors, 1)
		assert.Equal(t, "PRODUCT_USER_EXTERNAL_ID_ALREADY_EXISTS", resp.JSON409.Errors[0].Code)
	})
}

func TestCreateProductUserExternalIDIsScopedToTheProduct(t *testing.T) {
	t.Parallel()
	t.Run("Two products can each create a user with the same external ID", func(t *testing.T) {
		t.Parallel()
		first := createTestProductContext(t)
		second := createTestProductContext(t)
		externalID := newExternalID()

		firstUser := first.ProductUsers().Create(newProductUserBody(&externalID))
		secondUser := second.ProductUsers().Create(newProductUserBody(&externalID))

		assert.NotEqual(t, firstUser.Id, secondUser.Id)
		assert.Equal(t, externalID, *secondUser.ExternalId)
	})
}

func TestCreateProductUserExternalIDIsCaseSensitive(t *testing.T) {
	t.Parallel()
	t.Run("External IDs that differ only in letter case name two different users", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		upper := "USR_" + strings.ToUpper(uuid.NewString())
		lower := strings.ToLower(upper)

		upperUser := product.ProductUsers().Create(newProductUserBody(&upper))
		lowerUser := product.ProductUsers().Create(newProductUserBody(&lower))

		assert.NotEqual(t, upperUser.Id, lowerUser.Id)
		assert.Equal(t, upper, *upperUser.ExternalId)
		assert.Equal(t, lower, *lowerUser.ExternalId)
	})
}

func TestCreateProductUserRefusesAnEmptyExternalID(t *testing.T) {
	t.Parallel()
	t.Run("An empty external ID is a bad request", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp := product.ProductUsers().CreateRaw(newProductUserBody(new("")))

		requireExternalIDRefused(t, resp, "notblank")
	})
}

func TestCreateProductUserRefusesABlankExternalID(t *testing.T) {
	t.Parallel()
	t.Run("An external ID of only spaces is a bad request", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)

		resp := product.ProductUsers().CreateRaw(newProductUserBody(new("   ")))

		requireExternalIDRefused(t, resp, "notblank")
	})
}

func TestCreateProductUserRefusesAnExternalIDLongerThan255Characters(t *testing.T) {
	t.Parallel()
	t.Run("An external ID of 256 characters is a bad request", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		tooLong := strings.Repeat("x", productUserExternalIDMaxLength+1)

		resp := product.ProductUsers().CreateRaw(newProductUserBody(&tooLong))

		requireExternalIDRefused(t, resp, "max")
	})
}

func TestCreateProductUserAcceptsAnExternalIDOf255Characters(t *testing.T) {
	t.Parallel()
	t.Run("An external ID of exactly 255 characters is stored unchanged", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		longest := newExternalID()
		longest += strings.Repeat("x", productUserExternalIDMaxLength-len(longest))

		created := product.ProductUsers().Create(newProductUserBody(&longest))

		require.NotNil(t, created.ExternalId)
		assert.Equal(t, longest, *created.ExternalId)
	})
}

func TestProductUserExternalIDCannotBeChangedAfterCreate(t *testing.T) {
	t.Parallel()
	t.Run("No update method changes a product user's external ID", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		externalID := newExternalID()
		created := product.ProductUsers().Create(newProductUserBody(&externalID))
		replacement := newExternalID()

		for _, method := range []string{http.MethodPatch, http.MethodPut} {
			resp := product.ProductUsers().SendRaw(method, created.Id, map[string]string{"external_id": replacement})
			assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, "%s: %s", method, resp.Body)
		}

		fetched := product.ProductUsers().Get(created.Id)
		require.NotNil(t, fetched.ExternalId)
		assert.Equal(t, externalID, *fetched.ExternalId)
		decision := product.Access().User(replacement).InOrganization(product.CreateOrganization(
			t, "Org-"+uuid.NewString(), nil,
		).Id).Decide()
		requireDenied(t, decision, ct.SUBJECTNOTSYNCED)
	})
}

func TestClerkIngestOfAProductCreatedUser(t *testing.T) {
	t.Parallel()
	t.Run("A Clerk event for a user the product created with its Clerk user ID updates that user", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		createActiveClerkIntegrationInstance(t, product)
		clerkUserID := "user_clerk_" + itshared.Faker.UUID().V4()
		created := product.ProductUsers().Create(newProductUserBody(&clerkUserID))
		syncedEmail := itshared.Faker.Internet().Email()

		resp := sendClerkWebhook(
			t, product.ProductID,
			clerkUserCreatedPayload(t, clerkUserID, syncedEmail, "Synced", "User"),
			clerkTestWebhookSecret,
		)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))

		require.Eventually(t, func() bool {
			return product.ProductUsers().Get(created.Id).Email == syncedEmail
		}, 10*time.Second, 200*time.Millisecond)
		users := searchProductUsers(t, product).JSON200
		assert.Equal(t, int64(1), users.Total)
		synced := product.ProductUsers().Get(created.Id)
		require.NotNil(t, synced.ExternalId)
		assert.Equal(t, clerkUserID, *synced.ExternalId)
		require.NotNil(t, synced.Name)
		assert.Equal(t, "Synced User", *synced.Name)
	})
}

func TestAnchorSDKCreatesAProductUserWithAnExternalID(t *testing.T) {
	t.Parallel()
	t.Run("The SDK creates a user with an external ID that access evaluation then names", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		organizationID, role, permission := readerMembership(t, product)
		sdk := product.SDKClient()
		externalID := newExternalID()

		user, err := sdk.Users().Create(itshared.Faker.Internet().Email()).
			Name("SDK User").
			ExternalID(externalID).
			Do(t.Context())
		require.NoError(t, err)
		require.NotNil(t, user.ExternalId)
		assert.Equal(t, externalID, *user.ExternalId)
		_, err = sdk.Organization(organizationID).Members().Add(t.Context(), user.Id, role.ID)
		require.NoError(t, err)

		decision, err := sdk.AccessForUser(externalID).
			InOrganization(organizationID).
			Permissions(permission).
			Do(t.Context())
		require.NoError(t, err)
		assert.True(t, decision.Decision)

		_, err = sdk.Users().Create(itshared.Faker.Internet().Email()).
			Name("SDK Duplicate").
			ExternalID(externalID).
			Do(t.Context())
		assert.ErrorIs(t, err, anchorsdk.ErrConflict)
	})
}
