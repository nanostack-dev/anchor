package ct_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
)

// givenSuspendedMember creates a product user suspended on create, with an
// external ID, and makes it a member of organization A holding the world's role.
func (w accessWorld) givenSuspendedMember(t *testing.T) (ct.ProductUserResponse, string) {
	t.Helper()
	externalID := "user_clerk_" + itshared.Faker.UUID().V4()
	name := itshared.Faker.Person().Name()
	status := ct.Suspended
	client, _ := w.product.CreateAPIKeyClientWithScopes([]string{"product_user:create"})
	resp, err := client.CreateProductUserWithResponse(context.Background(), w.product.ProductID,
		ct.CreateProductUserJSONRequestBody{
			Email: itshared.Faker.Internet().Email(), Name: &name, Status: &status, ExternalId: &externalID,
		})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode(), string(resp.Body))
	createDSLMembership(t, w.product, resp.JSON201.Id, w.orgA.Id, w.role.ID)
	return *resp.JSON201, externalID
}

func TestAccessEvaluationSuspendedUser(t *testing.T) {
	t.Parallel()

	t.Run("A suspended member holding the permission is denied as suspended", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		_, externalID := w.givenSuspendedMember(t)

		response := w.access.User(externalID).InOrganization(w.orgA.Id).Can(w.read).Decide()

		requireDenied(t, response, ct.SUBJECTSUSPENDED)
		require.Len(t, response.Evaluations, 1)
		assert.False(t, response.Evaluations[0].Decision)
		assert.Nil(t, response.Context.ProductUser, "a refused subject reaches no identity")
	})

	t.Run("A Clerk update of a suspended user keeps it suspended and denied", func(t *testing.T) {
		t.Parallel()
		w := newAccessWorld(t)
		createActiveClerkIntegrationInstance(t, w.product)
		user, externalID := w.givenSuspendedMember(t)

		updatedEmail := itshared.Faker.Internet().Email()
		resp := sendClerkWebhook(t, w.product.ProductID,
			clerkUserUpdatedPayload(t, externalID, updatedEmail, "Renamed", "In Clerk"), clerkTestWebhookSecret)
		require.Equal(t, http.StatusOK, resp.StatusCode(), string(resp.Body))

		reader, _ := w.product.CreateAPIKeyClientWithScopes([]string{"product_user:read"})
		var got ct.ProductUserResponse
		require.Eventually(t, func() bool {
			read, err := reader.GetProductUserWithResponse(context.Background(), w.product.ProductID, user.Id)
			if err != nil || read.JSON200 == nil {
				return false
			}
			got = *read.JSON200
			return got.Email == updatedEmail
		}, 10*time.Second, 200*time.Millisecond, "Clerk's email reaches the user")
		assert.Equal(t, "Renamed In Clerk", *got.Name)
		assert.Equal(t, ct.Suspended, got.Status, "a Clerk sync does not lift a suspension")

		response := w.access.User(externalID).InOrganization(w.orgA.Id).Can(w.read).Decide()
		requireDenied(t, response, ct.SUBJECTSUSPENDED)
	})
}
