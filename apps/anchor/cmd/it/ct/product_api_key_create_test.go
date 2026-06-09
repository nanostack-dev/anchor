package ct_test

import (
	"strings"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"

	itshared "anchor/cmd/it/shared"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestProductAPIKeyCreate(t *testing.T) {
	product := createTestProductContext(t)
	permission1 := "organization:read"
	permission2 := "organization:create"
	now := time.Now()
	t.Run(
		"Create API Key with Permissions", func(t *testing.T) {
			apiKeyName := uuid.NewString()
			description := itshared.Faker.Lorem().Sentence(5)
			resp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				t.Context(),
				product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Description: &description,
					Name:        apiKeyName,
					Permissions: []string{
						permission1,
						permission2,
					},
				},
			)
			if err != nil {
				t.Fatalf("failed to create API key: %v", err)
			}
			if assert.NotNil(t, resp.JSON201) {
				response := resp.JSON201
				assert.NotEmpty(t, response.Value)
				assert.NotEmpty(t, response.Id)
				assert.Equal(t, product.ProductID, response.ProductId)
				assert.Equal(
					t, []ct.ProductAPIKeyPermissionResponse{
						{
							PermissionName:  permission1,
							ProductApiKeyId: response.Id,
							ProductId:       product.ProductID,
						},
						{
							PermissionName:  permission2,
							ProductApiKeyId: response.Id,
							ProductId:       product.ProductID,
						},
					}, response.Permissions,
				)
				assert.WithinDuration(t, now, response.CreatedAt, time.Second*5)
				assert.WithinDuration(t, now, response.UpdatedAt, time.Second*5)
				assert.Equal(t, apiKeyName, response.Name)
				assert.Equal(t, description, *response.Description)
				assert.NotEmpty(t, response.ObfuscatedValue)
				assert.NotEqual(t, response.ObfuscatedValue, response.Value)
				assert.Equal(t, ct.ProductAPIKeyStatusACTIVE, response.Status)
				assert.False(t, response.Mutable)
			}
		},
	)
	t.Run(
		"Create mutable API key when requested", func(t *testing.T) {
			apiKeyName := uuid.NewString()
			mutable := true

			resp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
				t.Context(),
				product.ProductID,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        apiKeyName,
					Mutable:     &mutable,
					Permissions: []string{permission1},
				},
			)
			if err != nil {
				t.Fatalf("failed to create mutable API key: %v", err)
			}
			if assert.NotNil(t, resp.JSON201) {
				assert.True(t, resp.JSON201.Mutable)
			}
		},
	)
	t.Run(
		"Create API key keeps fixed Anchor prefix when organization prefix is configured", func(t *testing.T) {
			prefix := "acme"
			createProductResp, err := testOwnerClient(t).CreateProductWithResponse(
				t.Context(),
				ct.CreateProductJSONRequestBody{
					Name: "API Prefix Product " + uuid.NewString(),
					Config: &ct.ProductConfigRequest{OrganizationApiKeys: &ct.ProductOrganizationAPIKeysConfigRequest{
						Prefix: prefix,
					}},
				},
			)
			if err != nil {
				t.Fatalf("failed to create product: %v", err)
			}
			if !assert.NotNil(t, createProductResp.JSON201) {
				return
			}

			resp, err := testOwnerClient(t).CreateProductAPIKeyWithResponse(
				t.Context(),
				createProductResp.JSON201.Id,
				ct.CreateProductAPIKeyJSONRequestBody{
					Name:        uuid.NewString(),
					Permissions: []string{permission1},
				},
			)
			if err != nil {
				t.Fatalf("failed to create API key: %v", err)
			}
			if assert.NotNil(t, resp.JSON201) {
				assert.True(t, strings.HasPrefix(resp.JSON201.Value, "anchor_prd_apikey_"))
				assert.True(t, strings.HasPrefix(resp.JSON201.ObfuscatedValue, "anchor_prd_apikey_"))
				assert.False(t, strings.HasPrefix(resp.JSON201.Value, prefix+"_prd_apikey_"))
			}
		},
	)
	t.Run(
		"Name validation cases should trigger 400 Bad Request", func(t *testing.T) {
			testCases := []struct {
				name        string
				inputName   string
				expectedMsg string
			}{
				{
					name:        "Name is null",
					inputName:   "",
					expectedMsg: "Name is a required field",
				},
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
						body := ct.CreateProductAPIKeyJSONRequestBody{
							Permissions: []string{
								permission1,
								permission2,
							},
						}
						body.Name = tc.inputName
						resp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
							t.Context(),
							product.ProductID,
							body,
						)
						if err != nil {
							t.Fatalf("failed to create API key: %v", err)
						}
						if assert.NotNil(t, resp.JSON400) {
							AssertAPIError(
								t, resp.JSON400.Errors, ct.ApiError{
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
			testCases := []struct {
				name        string
				inputDesc   *string
				expectedMsg string
			}{
				{
					name:        "Description is null (should be allowed)",
					inputDesc:   nil,
					expectedMsg: "",
				},
				{
					name:        "Description is blank (should be allowed)",
					inputDesc:   ptr.Ptr("   "),
					expectedMsg: "",
				},
				{
					name:        "Description exceeds 500 characters",
					inputDesc:   ptr.Ptr(strings.Repeat("a", 501)),
					expectedMsg: "Description must be a maximum of 500 characters in length",
				},
			}
			for _, tc := range testCases {
				t.Run(
					tc.name, func(t *testing.T) {
						body := ct.CreateProductAPIKeyJSONRequestBody{
							Name: uuid.NewString(),
							Permissions: []string{
								permission1,
								permission2,
							},
						}

						body.Description = tc.inputDesc
						resp, err := product.OwnerAuthenticatedClient().CreateProductAPIKeyWithResponse(
							t.Context(),
							product.ProductID,
							body,
						)
						if err != nil {
							t.Fatalf("failed to create API key: %v", err)
						}
						if tc.expectedMsg == "" {
							assert.Nil(t, resp.JSON400)
						} else if assert.NotNil(t, resp.JSON400) {
							AssertAPIError(
								t, resp.JSON400.Errors, ct.ApiError{
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
}
