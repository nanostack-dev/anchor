package ct_test

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	nameRaceRounds  = 5
	nameRaceWriters = 8
)

type nameRaceOutcome struct {
	status   int
	conflict *ct.ApiErrorResponse
	err      error
}

type nameRaceWrite func(writer int) nameRaceOutcome

func raceOutcome(status int, conflict *ct.ApiErrorResponse, err error) nameRaceOutcome {
	return nameRaceOutcome{status: status, conflict: conflict, err: err}
}

// assertOneWinnerPerName runs nameRaceWriters concurrent writes of one name per
// round. A pre-check cannot see an uncommitted row, so losers regularly reach
// the unique constraint instead of the pre-check.
func assertOneWinnerPerName(
	t *testing.T,
	successStatus int,
	conflictCode string,
	prepareRound func(name string) nameRaceWrite,
) {
	t.Helper()
	for range nameRaceRounds {
		name := "Race-" + uuid.NewString()
		write := prepareRound(name)
		outcomes := make([]nameRaceOutcome, nameRaceWriters)
		var wg sync.WaitGroup
		for writer := range nameRaceWriters {
			wg.Go(func() { outcomes[writer] = write(writer) })
		}
		wg.Wait()

		winners := 0
		for _, outcome := range outcomes {
			require.NoError(t, outcome.err)
			if outcome.status == successStatus {
				winners++
				continue
			}
			require.Equal(t, http.StatusConflict, outcome.status)
			require.NotNil(t, outcome.conflict)
			require.Len(t, outcome.conflict.Errors, 1)
			assert.Equal(t, conflictCode, outcome.conflict.Errors[0].Code)
		}
		assert.Equal(t, 1, winners, "exactly one write of %q wins", name)
	}
}

// mixedCase lower-cases the name for odd writers, so a resource whose names
// are unique regardless of case also races on its case-insensitive index.
func mixedCase(name string, writer int) string {
	if writer%2 == 1 {
		return strings.ToLower(name)
	}
	return name
}

func raceProductUser(name string) ct.CreateProductUserJSONRequestBody {
	userName := "Race User"
	status := ct.Active
	return ct.CreateProductUserJSONRequestBody{
		Email:  strings.ToLower(name) + "@example.com",
		Name:   &userName,
		Status: &status,
	}
}

func sameNameCreate(create func(name string, writer int) nameRaceOutcome) func(string) nameRaceWrite {
	return func(name string) nameRaceWrite {
		return func(writer int) nameRaceOutcome { return create(name, writer) }
	}
}

// renameToSameName gives every writer its own resource, so the renames race
// only on the shared new name.
func renameToSameName(
	t *testing.T,
	create func(initialName string) string,
	rename func(id, name string, writer int) nameRaceOutcome,
) func(string) nameRaceWrite {
	return func(name string) nameRaceWrite {
		ids := make([]string, nameRaceWriters)
		for i := range ids {
			ids[i] = create("Initial-" + uuid.NewString())
			require.NotEmpty(t, ids[i])
		}
		return func(writer int) nameRaceOutcome { return rename(ids[writer], name, writer) }
	}
}

func TestConcurrentSameNameWritesReturnConflict(t *testing.T) {
	t.Parallel()

	t.Run(
		"Concurrent organization API key create requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
			org := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
			client, _ := product.CreateAPIKeyClientWithScopes([]string{"organization_api_key:create"})
			assertOneWinnerPerName(t, http.StatusCreated, "ORGANIZATION_API_KEY_NAME_DUPLICATE",
				sameNameCreate(func(name string, _ int) nameRaceOutcome {
					resp, err := client.CreateOrganizationAPIKeyWithResponse(
						t.Context(),
						product.ProductID,
						org.Id,
						ct.CreateOrganizationAPIKeyJSONRequestBody{
							Name:        name,
							Permissions: []string{permissions.FileRead},
						},
					)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent organization API key rename requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
			org := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
			client, _ := product.CreateAPIKeyClientWithScopes(
				[]string{"organization_api_key:create", "organization_api_key:update"},
			)
			assertOneWinnerPerName(t, http.StatusOK, "ORGANIZATION_API_KEY_NAME_DUPLICATE",
				renameToSameName(t, func(initialName string) string {
					resp, err := client.CreateOrganizationAPIKeyWithResponse(
						t.Context(), product.ProductID, org.Id,
						ct.CreateOrganizationAPIKeyJSONRequestBody{
							Name:        initialName,
							Permissions: []string{permissions.FileRead},
						},
					)
					require.NoError(t, err)
					require.Equal(t, http.StatusCreated, resp.StatusCode())
					return resp.JSON201.Id
				}, func(id, name string, _ int) nameRaceOutcome {
					resp, err := client.UpdateOrganizationAPIKeyWithResponse(
						t.Context(), product.ProductID, org.Id, id,
						ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: name},
					)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent product API key create requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			client := product.OwnerAuthenticatedClient()
			assertOneWinnerPerName(t, http.StatusCreated, "PRODUCT_API_KEY_NAME_DUPLICATE",
				sameNameCreate(func(name string, _ int) nameRaceOutcome {
					resp, err := client.CreateProductAPIKeyWithResponse(
						t.Context(), product.ProductID,
						ct.CreateProductAPIKeyJSONRequestBody{Name: name, Permissions: []string{"organization:read"}},
					)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent product API key rename requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			client := product.OwnerAuthenticatedClient()
			assertOneWinnerPerName(t, http.StatusOK, "PRODUCT_API_KEY_NAME_DUPLICATE",
				renameToSameName(t, func(initialName string) string {
					resp, err := client.CreateProductAPIKeyWithResponse(
						t.Context(), product.ProductID,
						ct.CreateProductAPIKeyJSONRequestBody{
							Name:        initialName,
							Permissions: []string{"organization:read"},
						},
					)
					require.NoError(t, err)
					require.Equal(t, http.StatusCreated, resp.StatusCode())
					return resp.JSON201.Id
				}, func(id, name string, _ int) nameRaceOutcome {
					resp, err := client.UpdateProductAPIKeyWithResponse(
						t.Context(), product.ProductID, id,
						ct.UpdateProductAPIKeyJSONRequestBody{Name: name},
					)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent product role create requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			client := product.OwnerAuthenticatedClient()
			assertOneWinnerPerName(t, http.StatusCreated, "ROLE_NAME_DUPLICATE",
				sameNameCreate(func(name string, writer int) nameRaceOutcome {
					resp, err := client.CreateProductRoleWithResponse(
						t.Context(),
						product.ProductID,
						ct.CreateProductRoleJSONRequestBody{
							ResourceType: "organization",
							Name:         mixedCase(name, writer),
						},
					)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent product role rename requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			client := product.OwnerAuthenticatedClient()
			assertOneWinnerPerName(t, http.StatusOK, "ROLE_NAME_DUPLICATE",
				renameToSameName(t, func(initialName string) string {
					resp, err := client.CreateProductRoleWithResponse(
						t.Context(),
						product.ProductID,
						ct.CreateProductRoleJSONRequestBody{ResourceType: "organization", Name: initialName},
					)
					require.NoError(t, err)
					require.Equal(t, http.StatusCreated, resp.StatusCode())
					return resp.JSON201.Id
				}, func(id, name string, writer int) nameRaceOutcome {
					resp, err := client.UpdateProductRoleWithResponse(
						t.Context(), product.ProductID, id,
						ct.UpdateProductRoleJSONRequestBody{Name: mixedCase(name, writer)},
					)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent product resource permission create requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			client := product.OwnerAuthenticatedClient()
			assertOneWinnerPerName(t, http.StatusCreated, "RESOURCE_PERMISSION_ALREADY_EXISTS",
				sameNameCreate(func(name string, writer int) nameRaceOutcome {
					resp, err := client.CreateProductResourcePermissionWithResponse(
						t.Context(),
						product.ProductID,
						ct.CreateProductResourcePermissionRequest{
							ResourceType: "organization",
							Name:         "race:" + mixedCase(name, writer),
						},
					)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent product rename requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			client := createTestProductContext(t).OwnerAuthenticatedClient()
			assertOneWinnerPerName(t, http.StatusOK, "PRODUCT_ALREADY_EXISTS",
				renameToSameName(t, func(initialName string) string {
					resp, err := client.CreateProductWithResponse(
						t.Context(), ct.CreateProductJSONRequestBody{Name: initialName},
					)
					require.NoError(t, err)
					require.Equal(t, http.StatusCreated, resp.StatusCode())
					return resp.JSON201.Id
				}, func(id, name string, writer int) nameRaceOutcome {
					resp, err := client.UpdateProductWithResponse(
						t.Context(), id, ct.UpdateProductJSONRequestBody{Name: mixedCase(name, writer)},
					)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent product user create requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			client, _ := product.CreateAPIKeyClientWithScopes([]string{"product_user:create"})
			assertOneWinnerPerName(t, http.StatusCreated, "PRODUCT_USER_EMAIL_ALREADY_EXISTS",
				sameNameCreate(func(name string, _ int) nameRaceOutcome {
					resp, err := client.CreateProductUserWithResponse(
						t.Context(), product.ProductID,
						raceProductUser(name),
					)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent product user create requests with one external ID produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			client, _ := product.CreateAPIKeyClientWithScopes([]string{"product_user:create"})
			assertOneWinnerPerName(t, http.StatusCreated, "PRODUCT_USER_EXTERNAL_ID_ALREADY_EXISTS",
				sameNameCreate(func(name string, writer int) nameRaceOutcome {
					body := raceProductUser(name + "-" + strconv.Itoa(writer))
					body.ExternalId = &name
					resp, err := client.CreateProductUserWithResponse(t.Context(), product.ProductID, body)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent organization member add requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			org := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
			client := product.AllScopeAPIKeyClient()
			roleResp, err := product.OwnerAuthenticatedClient().CreateProductRoleWithResponse(
				t.Context(),
				product.ProductID, ct.CreateProductRoleJSONRequestBody{ResourceType: "organization", Name: "Role-" + uuid.NewString()},
			)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, roleResp.StatusCode())
			assertOneWinnerPerName(t, http.StatusCreated, "ORGANIZATION_MEMBERSHIP_ALREADY_EXISTS",
				func(name string) nameRaceWrite {
					userResp, userErr := client.CreateProductUserWithResponse(
						t.Context(), product.ProductID,
						raceProductUser(name),
					)
					require.NoError(t, userErr)
					require.Equal(t, http.StatusCreated, userResp.StatusCode())
					return func(int) nameRaceOutcome {
						resp, addErr := client.AddOrganizationMemberWithResponse(
							t.Context(), product.ProductID, org.Id,
							ct.AddOrganizationMemberJSONRequestBody{
								ProductUserId: userResp.JSON201.Id,
								RoleId:        new(roleResp.JSON201.Id),
							},
						)
						if addErr != nil {
							return raceOutcome(0, nil, addErr)
						}
						return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
					}
				})
		},
	)

	t.Run(
		"Concurrent email template create requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			tc := newEmailTestCtx(t)
			client := tc.product.OwnerAuthenticatedClient()
			assertOneWinnerPerName(t, http.StatusCreated, "EMAIL_TEMPLATE_SLUG_TAKEN",
				func(string) nameRaceWrite {
					slug := uniqueSlug()
					return func(int) nameRaceOutcome {
						resp, err := client.CreateEmailTemplateWithResponse(
							t.Context(), tc.product.ProductID,
							ct.CreateEmailTemplateJSONRequestBody{
								Slug:     slug,
								Name:     "Race",
								Subject:  "S",
								BodyHtml: "<p>B</p>",
							},
						)
						if err != nil {
							return raceOutcome(0, nil, err)
						}
						return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
					}
				})
		},
	)

	t.Run(
		"Concurrent resource type create requests with one slug produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			product := createTestProductContext(t)
			client := product.AllScopeAPIKeyClient()
			assertOneWinnerPerName(t, http.StatusCreated, "RESOURCE_TYPE_SLUG_DUPLICATE",
				sameNameCreate(func(name string, _ int) nameRaceOutcome {
					slug := strings.ReplaceAll(strings.ToLower(name), "-", "_")
					resp, err := client.CreateProductResourceTypeWithResponse(
						t.Context(), product.ProductID,
						ct.CreateProductResourceTypeJSONRequestBody{
							Slug: slug, Name: name, ParentTypes: []string{"organization"},
						},
					)
					if err != nil {
						return raceOutcome(0, nil, err)
					}
					return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
				}))
		},
	)

	t.Run(
		"Concurrent integration instance create requests produce one success and conflicts for all other writers",
		func(t *testing.T) {
			t.Parallel()
			body := ct.CreateIntegrationInstanceJSONRequestBody{}
			require.NoError(t, body.FromClerkIntegrationInstanceCreateRequest(
				ct.ClerkIntegrationInstanceCreateRequest{ProviderType: "CLERK"},
			))
			assertOneWinnerPerName(t, http.StatusCreated, "INTEGRATION_INSTANCE_ALREADY_EXISTS",
				func(string) nameRaceWrite {
					product := createTestProductContext(t)
					client := product.OwnerAuthenticatedClient()
					return func(int) nameRaceOutcome {
						resp, err := client.CreateIntegrationInstanceWithResponse(t.Context(), product.ProductID, body)
						if err != nil {
							return raceOutcome(0, nil, err)
						}
						return raceOutcome(resp.StatusCode(), resp.JSON409, nil)
					}
				})
		},
	)
}
