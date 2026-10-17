package ct_test

import (
	"context"
	"encoding/json"
	"net/http"

	ct "github.com/nanostack-dev/anchor/clients/go"

	itshared "anchor/cmd/it/shared"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateProductUser(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run(
		"Creating a product user succeeds", func(t *testing.T) {
			productContext := createTestProductContext(t)

			email := itshared.Faker.Internet().Email()
			name := itshared.Faker.Person().Name()
			status := ct.Active

			apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:create"})

			resp, err := apiKeyClient.CreateProductUserWithResponse(
				ctx, productContext.ProductID,
				ct.CreateProductUserJSONRequestBody{
					Email:  email,
					Name:   &name,
					Status: &status,
				},
			)

			require.NoError(t, err, "create product user request should not error")
			assert.Equal(
				t, http.StatusCreated,
				resp.StatusCode(),
				"create product user should return 201 Created",
			)
			assert.NotNil(t, resp.JSON201)
			assert.Equal(t, resp.JSON201.Email, email)
			assert.Equal(t, *resp.JSON201.Name, name)
			assert.Equal(t, ct.Active, resp.JSON201.Status)
			assert.NotEmpty(t, resp.JSON201.Id)
			assert.NotZero(t, resp.JSON201.CreatedAt)
			assert.NotZero(t, resp.JSON201.UpdatedAt)
		},
	)

	t.Run(
		"Creating a product user with invalid email returns a bad request", func(t *testing.T) {
			productContext := createTestProductContext(t)

			invalidEmail := "invalid-email"
			name := itshared.Faker.Person().Name()
			status := ct.Active

			apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:create"})

			resp, err := apiKeyClient.CreateProductUserWithResponse(
				ctx, productContext.ProductID,
				ct.CreateProductUserJSONRequestBody{
					Email:  invalidEmail,
					Name:   &name,
					Status: &status,
				},
			)

			require.NoError(
				t, err, "create product user with invalid email should not error",
			)
			assert.Equal(
				t, 400, resp.StatusCode(),
				"create product user with invalid email should return 400 Bad Request",
			)
			assert.NotNil(t, resp.JSON400)
			assert.Contains(t, resp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
			assert.Contains(t, resp.JSON400.Errors[0].Message, "email")
		},
	)

	t.Run(
		"Creating a product user with missing name returns a bad request", func(t *testing.T) {
			productContext := createTestProductContext(t)

			email := itshared.Faker.Internet().Email()
			status := ct.Active

			apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:create"})

			resp, err := apiKeyClient.CreateProductUserWithResponse(
				ctx, productContext.ProductID,
				ct.CreateProductUserJSONRequestBody{
					Email:  email,
					Status: &status,
				},
			)

			require.NoError(
				t, err, "create product user with missing name should not error",
			)
			assert.Equal(
				t, 400, resp.StatusCode(),
				"create product user with missing name should return 400 Bad Request",
			)
			assert.NotNil(t, resp.JSON400)
			assert.Contains(t, resp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
			assert.Contains(t, resp.JSON400.Errors[0].Message, "Name is a required field")
		},
	)

	t.Run(
		"Creating a product user with empty name returns a bad request", func(t *testing.T) {
			productContext := createTestProductContext(t)

			email := itshared.Faker.Internet().Email()
			emptyName := ""
			status := ct.Active

			apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:create"})

			resp, err := apiKeyClient.CreateProductUserWithResponse(
				ctx, productContext.ProductID,
				ct.CreateProductUserJSONRequestBody{
					Email:  email,
					Name:   &emptyName,
					Status: &status,
				},
			)

			require.NoError(
				t, err, "create product user with empty name should not error",
			)
			assert.Equal(
				t, 400, resp.StatusCode(),
				"create product user with empty name should return 400 Bad Request",
			)
			assert.NotNil(t, resp.JSON400)
			assert.Contains(t, resp.JSON400.Errors[0].Code, "VALIDATION_ERROR")
			assert.Contains(t, resp.JSON400.Errors[0].Message, "Name is a required field")
		},
	)

	t.Run(
		"Creating a product user with inactive status succeeds", func(t *testing.T) {
			productContext := createTestProductContext(t)

			email := itshared.Faker.Internet().Email()
			name := itshared.Faker.Person().Name()
			status := ct.Suspended

			apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:create"})

			resp, err := apiKeyClient.CreateProductUserWithResponse(
				ctx, productContext.ProductID,
				ct.CreateProductUserJSONRequestBody{
					Email:  email,
					Name:   &name,
					Status: &status,
				},
			)

			require.NoError(
				t, err, "create product user with inactive status should not error",
			)
			assert.Equal(
				t, http.StatusCreated,
				resp.StatusCode(),
				"create product user with inactive status should return 201 Created",
			)
			assert.NotNil(t, resp.JSON201)
			assert.Equal(t, resp.JSON201.Email, email)
			assert.Equal(t, *resp.JSON201.Name, name)
			assert.Equal(t, ct.Suspended, resp.JSON201.Status)
		},
	)

	t.Run(
		"Creating a product user with an existing email returns a conflict", func(t *testing.T) {
			productContext := createTestProductContext(t)

			email := itshared.Faker.Internet().Email()
			name1 := itshared.Faker.Person().Name()
			name2 := itshared.Faker.Person().Name()
			status := ct.Active

			resp1, err := func() *ct.ClientWithResponses {
				apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:create"})
				return apiKeyClient
			}().CreateProductUserWithResponse(
				ctx, productContext.ProductID,
				ct.CreateProductUserJSONRequestBody{
					Email:  email,
					Name:   &name1,
					Status: &status,
				},
			)

			require.NoError(t, err, "create first product user should not error")
			assert.Equal(
				t, http.StatusCreated,
				resp1.StatusCode(),
				"create first product user should return 201 Created",
			)

			resp2, err := func() *ct.ClientWithResponses {
				apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:create"})
				return apiKeyClient
			}().CreateProductUserWithResponse(
				ctx, productContext.ProductID,
				ct.CreateProductUserJSONRequestBody{
					Email:  email,
					Name:   &name2,
					Status: &status,
				},
			)

			require.NoError(t, err, "create duplicate product user should not error")
			assert.Equal(
				t, http.StatusConflict, resp2.StatusCode(),
			)
			var errResp ct.ApiErrorResponse
			require.NoError(t, json.Unmarshal(resp2.Body, &errResp))
			require.NotEmpty(t, errResp.Errors)
			assert.Contains(t, errResp.Errors[0].Code, "PRODUCT_USER_EMAIL_ALREADY_EXISTS")
			assert.Contains(
				t, errResp.Errors[0].Message,
				"A product user with this email already exists in this product",
			)
		},
	)

	t.Run("A successful change emits the corresponding signed product event", func(t *testing.T) {
		productContext := createTestProductContext(t)
		sink := productContext.CaptureEvents()
		apiKeyClient, _ := productContext.CreateAPIKeyClientWithScopes([]string{"product_user:create"})
		name := itshared.Faker.Person().Name()
		resp, err := apiKeyClient.CreateProductUserWithResponse(
			ctx, productContext.ProductID,
			ct.CreateProductUserJSONRequestBody{
				Email: itshared.Faker.Internet().Email(),
				Name:  &name,
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode())
		sink.WaitFor("product_user.created", map[string]string{"product_user_id": resp.JSON201.Id})
	})
}
