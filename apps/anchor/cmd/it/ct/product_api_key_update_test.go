package ct_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProductAPIKeyUpdate(t *testing.T) {
	ctx := context.Background()
	product := createTestProductContext(t)
	permission1 := "organization:read"
	permission2 := "organization:create"

	t.Run(
		"Update API key name and description successfully", func(t *testing.T) {
			apiKeyName := "OriginalKey_" + uuid.NewString()
			originalDescription := "Original description"

			createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Description: &originalDescription,
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)
			require.NotNil(t, createResp.JSON201)

			newName := "UpdatedKey_" + uuid.NewString()
			newDescription := "Updated description"

			updateResp, err := product.OwnerAuthenticatedClient().UpdateProductAPIKeyWithResponse(
				ctx, product.ProductID, createResp.JSON201.Id,
				ct.UpdateProductAPIKeyJSONRequestBody{
					Name:        newName,
					Description: &newDescription,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 200, updateResp.StatusCode())
			assert.NotNil(t, updateResp.JSON200)

			updatedKey := updateResp.JSON200
			assert.Equal(t, createResp.JSON201.Id, updatedKey.Id)
			assert.Equal(t, product.ProductID, updatedKey.ProductId)
			assert.Equal(t, newName, updatedKey.Name)
			assert.Equal(t, newDescription, *updatedKey.Description)
			assert.Equal(t, createResp.JSON201.ObfuscatedValue, updatedKey.ObfuscatedValue)
			assert.Equal(t, ct.ProductAPIKeyStatusACTIVE, updatedKey.Status)
			assert.Equal(t, createResp.JSON201.CreatedAt, updatedKey.CreatedAt)
			assert.True(t, updatedKey.UpdatedAt.After(createResp.JSON201.UpdatedAt))
		},
	)

	t.Run(
		"Update API key name only", func(t *testing.T) {
			apiKeyName := "NameOnlyKey_" + uuid.NewString()
			description := "Keep this description"

			createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Description: &description,
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)
			require.NotNil(t, createResp.JSON201)

			newName := "UpdatedNameOnly_" + uuid.NewString()

			updateResp, err := product.OwnerAuthenticatedClient().UpdateProductAPIKeyWithResponse(
				ctx, product.ProductID, createResp.JSON201.Id,
				ct.UpdateProductAPIKeyJSONRequestBody{
					Name: newName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 200, updateResp.StatusCode())
			assert.NotNil(t, updateResp.JSON200)

			updatedKey := updateResp.JSON200
			assert.Equal(t, newName, updatedKey.Name)
			assert.Equal(t, description, *updatedKey.Description)
		},
	)

	t.Run(
		"Update API key description only", func(t *testing.T) {
			apiKeyName := "DescOnlyKey_" + uuid.NewString()
			originalDescription := "Original description"

			createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Description: &originalDescription,
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)
			require.NotNil(t, createResp.JSON201)

			newDescription := "Updated description only"

			// PUT is a full representation: resend the existing name unchanged.
			updateResp, err := product.OwnerAuthenticatedClient().UpdateProductAPIKeyWithResponse(
				ctx, product.ProductID, createResp.JSON201.Id,
				ct.UpdateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Description: &newDescription,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 200, updateResp.StatusCode())
			assert.NotNil(t, updateResp.JSON200)

			updatedKey := updateResp.JSON200
			assert.Equal(t, apiKeyName, updatedKey.Name)
			assert.Equal(t, newDescription, *updatedKey.Description)
		},
	)

	t.Run(
		"Immutable API key can update name but cannot update permissions", func(t *testing.T) {
			apiKeyName := "ImmutableKey_" + uuid.NewString()

			createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, createResp.StatusCode())
			require.NotNil(t, createResp.JSON201)
			assert.False(t, createResp.JSON201.Mutable)

			newName := "ImmutableRenamed_" + uuid.NewString()
			nameUpdateResp, err := product.OwnerAuthenticatedClient().UpdateProductAPIKeyWithResponse(
				ctx, product.ProductID, createResp.JSON201.Id,
				ct.UpdateProductAPIKeyJSONRequestBody{
					Name: newName,
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, nameUpdateResp.StatusCode())
			require.NotNil(t, nameUpdateResp.JSON200)
			assert.Equal(t, newName, nameUpdateResp.JSON200.Name)
			assert.False(t, nameUpdateResp.JSON200.Mutable)

			updatedPermissions := []string{permission2}
			permissionUpdateResp, err := product.OwnerAuthenticatedClient().UpdateProductAPIKeyWithResponse(
				ctx, product.ProductID, createResp.JSON201.Id,
				ct.UpdateProductAPIKeyJSONRequestBody{
					Name:        newName,
					Permissions: &updatedPermissions,
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, permissionUpdateResp.StatusCode())
			if assert.NotNil(t, permissionUpdateResp.JSON400) {
				AssertAPIError(
					t, permissionUpdateResp.JSON400.Errors, ct.ApiError{
						Code:    "PRODUCT_API_KEY_PERMISSIONS_IMMUTABLE",
						Message: "Product API key permissions are immutable",
					},
				)
			}
		},
	)

	t.Run(
		"Mutable API key can update permissions", func(t *testing.T) {
			apiKeyName := "MutableKey_" + uuid.NewString()
			mutable := true

			createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Mutable:     &mutable,
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, createResp.StatusCode())
			require.NotNil(t, createResp.JSON201)
			assert.True(t, createResp.JSON201.Mutable)

			updatedPermissions := []string{permission2}
			updateResp, err := product.OwnerAuthenticatedClient().UpdateProductAPIKeyWithResponse(
				ctx, product.ProductID, createResp.JSON201.Id,
				ct.UpdateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Permissions: &updatedPermissions,
				},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, updateResp.StatusCode())
			require.NotNil(t, updateResp.JSON200)
			assert.True(t, updateResp.JSON200.Mutable)
			require.Len(t, updateResp.JSON200.Permissions, 1)
			assert.Equal(t, permission2, updateResp.JSON200.Permissions[0].PermissionName)
		},
	)

	t.Run(
		"Update non-existent API key returns 404", func(t *testing.T) {
			nonExistentID := ids.MustNew("product_apikey")
			newName := "NewName"

			updateResp, err := product.OwnerAuthenticatedClient().UpdateProductAPIKeyWithResponse(
				ctx, product.ProductID, nonExistentID,
				ct.UpdateProductAPIKeyJSONRequestBody{
					Name: newName,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 404, updateResp.StatusCode())
			assert.Nil(t, updateResp.JSON200)
		},
	)

	t.Run(
		"Update API key with duplicate name returns 400", func(t *testing.T) {
			key1Name := "FirstKey_" + uuid.NewString()
			key2Name := "SecondKey_" + uuid.NewString()

			createResp1, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        key1Name,
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp1.StatusCode(),
			)

			createResp2, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        key2Name,
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp2.StatusCode(),
			)

			updateResp, err := product.OwnerAuthenticatedClient().UpdateProductAPIKeyWithResponse(
				ctx, product.ProductID, createResp2.JSON201.Id,
				ct.UpdateProductAPIKeyJSONRequestBody{
					Name: key1Name,
				},
			)
			require.NoError(t, err)
			assert.Equal(t, 400, updateResp.StatusCode())
			assert.NotNil(t, updateResp.JSON400)
		},
	)

	t.Run(
		"Name validation cases should trigger 400 Bad Request", func(t *testing.T) {
			apiKeyName := "ValidationKey_" + uuid.NewString()

			createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)
			require.NotNil(t, createResp.JSON201)

			testCases := []struct {
				name        string
				inputName   string
				expectedMsg string
			}{
				{
					name:        "Name is blank",
					inputName:   "   ",
					expectedMsg: "Name cannot be blank",
				},
				{
					name:        "Name exceeds 100 characters",
					inputName:   strings.Repeat("a", 101),
					expectedMsg: "Name must be a maximum of 100 characters in length",
				},
			}

			for _, tc := range testCases {
				t.Run(
					tc.name, func(t *testing.T) {
						updateResp, nameValidationErr := product.OwnerAuthenticatedClient().
							UpdateProductAPIKeyWithResponse(
								ctx,
								product.ProductID,
								createResp.JSON201.Id,
								ct.UpdateProductAPIKeyJSONRequestBody{
									Name: tc.inputName,
								},
							)
						require.NoError(t, nameValidationErr)
						assert.Equal(t, 400, updateResp.StatusCode())
						if assert.NotNil(t, updateResp.JSON400) {
							AssertAPIError(
								t, updateResp.JSON400.Errors, ct.ApiError{
									Code:    "VALIDATION_ERROR",
									Message: tc.expectedMsg,
								},
							)
						}
					},
				)
			}
		},
	)

	t.Run(
		"Description validation cases should trigger 400 Bad Request", func(t *testing.T) {
			apiKeyName := "DescValidationKey_" + uuid.NewString()

			createResp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				ctx, product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Permissions: []string{permission1},
				},
			)
			require.NoError(t, err)
			require.Equal(
				t, http.StatusCreated,
				createResp.StatusCode(),
			)
			require.NotNil(t, createResp.JSON201)

			testCases := []struct {
				name        string
				inputDesc   *string
				expectedMsg string
			}{
				{
					name:        "Description is null should be allowed",
					inputDesc:   nil,
					expectedMsg: "",
				},
				{
					name:        "Description is blank should be allowed",
					inputDesc:   new("   "),
					expectedMsg: "",
				},
				{
					name:        "Description exceeds 500 characters",
					inputDesc:   new(strings.Repeat("a", 501)),
					expectedMsg: "Description must be a maximum of 500 characters in length",
				},
			}

			for _, tc := range testCases {
				t.Run(
					tc.name, func(t *testing.T) {
						updateResp, descValidationErr := product.OwnerAuthenticatedClient().
							UpdateProductAPIKeyWithResponse(
								ctx,
								product.ProductID,
								createResp.JSON201.Id,
								ct.UpdateProductAPIKeyJSONRequestBody{
									Name:        apiKeyName,
									Description: tc.inputDesc,
								},
							)
						require.NoError(t, descValidationErr)

						if tc.expectedMsg == "" {
							assert.Equal(t, 200, updateResp.StatusCode())
							assert.Nil(t, updateResp.JSON400)
						} else {
							assert.Equal(t, 400, updateResp.StatusCode())
							if assert.NotNil(t, updateResp.JSON400) {
								AssertAPIError(
									t, updateResp.JSON400.Errors, ct.ApiError{
										Code:    "VALIDATION_ERROR",
										Message: tc.expectedMsg,
									},
								)
							}
						}
					},
				)
			}
		},
	)
}
