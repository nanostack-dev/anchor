package ct_test

import (
	"net/http"
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

func sameNameCreate(create func(name string) nameRaceOutcome) func(string) nameRaceWrite {
	return func(name string) nameRaceWrite {
		return func(int) nameRaceOutcome { return create(name) }
	}
}

// renameToSameName gives every writer its own resource, so the renames race
// only on the shared new name.
func renameToSameName(
	t *testing.T,
	create func(initialName string) string,
	rename func(id, name string) nameRaceOutcome,
) func(string) nameRaceWrite {
	return func(name string) nameRaceWrite {
		ids := make([]string, nameRaceWriters)
		for i := range ids {
			ids[i] = create("Initial-" + uuid.NewString())
			require.NotEmpty(t, ids[i])
		}
		return func(writer int) nameRaceOutcome { return rename(ids[writer], name) }
	}
}

func TestConcurrentSameNameWritesReturnConflict(t *testing.T) {
	t.Parallel()

	t.Run("Organization API key create", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
		org := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
		client, _ := product.CreateAPIKeyClientWithScopes([]string{"organization_api_key:create"})
		assertOneWinnerPerName(t, http.StatusCreated, "ORGANIZATION_API_KEY_NAME_DUPLICATE",
			sameNameCreate(func(name string) nameRaceOutcome {
				resp, err := client.CreateOrganizationAPIKeyWithResponse(
					t.Context(), product.ProductID, org.Id,
					ct.CreateOrganizationAPIKeyJSONRequestBody{Name: name, Permissions: []string{permissions.FileRead}},
				)
				if err != nil {
					return nameRaceOutcome{err: err}
				}
				return nameRaceOutcome{status: resp.StatusCode(), conflict: resp.JSON409}
			}))
	})

	t.Run("Organization API key rename", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
		org := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
		client, _ := product.CreateAPIKeyClientWithScopes([]string{"organization_api_key:create", "organization_api_key:update"})
		assertOneWinnerPerName(t, http.StatusOK, "ORGANIZATION_API_KEY_NAME_DUPLICATE",
			renameToSameName(t, func(initialName string) string {
				resp, err := client.CreateOrganizationAPIKeyWithResponse(
					t.Context(), product.ProductID, org.Id,
					ct.CreateOrganizationAPIKeyJSONRequestBody{Name: initialName, Permissions: []string{permissions.FileRead}},
				)
				require.NoError(t, err)
				require.Equal(t, http.StatusCreated, resp.StatusCode())
				return resp.JSON201.Id
			}, func(id, name string) nameRaceOutcome {
				resp, err := client.UpdateOrganizationAPIKeyWithResponse(
					t.Context(), product.ProductID, org.Id, id,
					ct.UpdateOrganizationAPIKeyJSONRequestBody{Name: name},
				)
				if err != nil {
					return nameRaceOutcome{err: err}
				}
				return nameRaceOutcome{status: resp.StatusCode(), conflict: resp.JSON409}
			}))
	})

	t.Run("Product API key create", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()
		assertOneWinnerPerName(t, http.StatusCreated, "PRODUCT_API_KEY_NAME_DUPLICATE",
			sameNameCreate(func(name string) nameRaceOutcome {
				resp, err := client.CreateProductAPIKeyWithResponse(
					t.Context(), product.ProductID,
					ct.CreateProductAPIKeyJSONRequestBody{Name: name, Permissions: []string{"organization:read"}},
				)
				if err != nil {
					return nameRaceOutcome{err: err}
				}
				return nameRaceOutcome{status: resp.StatusCode(), conflict: resp.JSON409}
			}))
	})

	t.Run("Product API key rename", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()
		assertOneWinnerPerName(t, http.StatusOK, "PRODUCT_API_KEY_NAME_DUPLICATE",
			renameToSameName(t, func(initialName string) string {
				resp, err := client.CreateProductAPIKeyWithResponse(
					t.Context(), product.ProductID,
					ct.CreateProductAPIKeyJSONRequestBody{Name: initialName, Permissions: []string{"organization:read"}},
				)
				require.NoError(t, err)
				require.Equal(t, http.StatusCreated, resp.StatusCode())
				return resp.JSON201.Id
			}, func(id, name string) nameRaceOutcome {
				resp, err := client.UpdateProductAPIKeyWithResponse(
					t.Context(), product.ProductID, id,
					ct.UpdateProductAPIKeyJSONRequestBody{Name: name},
				)
				if err != nil {
					return nameRaceOutcome{err: err}
				}
				return nameRaceOutcome{status: resp.StatusCode(), conflict: resp.JSON409}
			}))
	})

	t.Run("Product role create", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()
		assertOneWinnerPerName(t, http.StatusCreated, "ROLE_NAME_DUPLICATE",
			sameNameCreate(func(name string) nameRaceOutcome {
				resp, err := client.CreateProductRoleWithResponse(
					t.Context(), product.ProductID, ct.CreateProductRoleJSONRequestBody{Name: name},
				)
				if err != nil {
					return nameRaceOutcome{err: err}
				}
				return nameRaceOutcome{status: resp.StatusCode(), conflict: resp.JSON409}
			}))
	})

	t.Run("Product role rename", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()
		assertOneWinnerPerName(t, http.StatusOK, "ROLE_NAME_DUPLICATE",
			renameToSameName(t, func(initialName string) string {
				resp, err := client.CreateProductRoleWithResponse(
					t.Context(), product.ProductID, ct.CreateProductRoleJSONRequestBody{Name: initialName},
				)
				require.NoError(t, err)
				require.Equal(t, http.StatusCreated, resp.StatusCode())
				return resp.JSON201.Id
			}, func(id, name string) nameRaceOutcome {
				resp, err := client.UpdateProductRoleWithResponse(
					t.Context(), product.ProductID, id, ct.UpdateProductRoleJSONRequestBody{Name: name},
				)
				if err != nil {
					return nameRaceOutcome{err: err}
				}
				return nameRaceOutcome{status: resp.StatusCode(), conflict: resp.JSON409}
			}))
	})

	t.Run("Product resource permission create", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		client := product.OwnerAuthenticatedClient()
		assertOneWinnerPerName(t, http.StatusCreated, "RESOURCE_PERMISSION_ALREADY_EXISTS",
			sameNameCreate(func(name string) nameRaceOutcome {
				resp, err := client.CreateProductResourcePermissionWithResponse(
					t.Context(), product.ProductID, ct.CreateProductResourcePermissionRequest{Name: "race:" + name},
				)
				if err != nil {
					return nameRaceOutcome{err: err}
				}
				return nameRaceOutcome{status: resp.StatusCode(), conflict: resp.JSON409}
			}))
	})

	t.Run("Product rename", func(t *testing.T) {
		t.Parallel()
		client := createTestProductContext(t).OwnerAuthenticatedClient()
		assertOneWinnerPerName(t, http.StatusOK, "PRODUCT_ALREADY_EXISTS",
			renameToSameName(t, func(initialName string) string {
				resp, err := client.CreateProductWithResponse(t.Context(), ct.CreateProductJSONRequestBody{Name: initialName})
				require.NoError(t, err)
				require.Equal(t, http.StatusCreated, resp.StatusCode())
				return resp.JSON201.Id
			}, func(id, name string) nameRaceOutcome {
				resp, err := client.UpdateProductWithResponse(t.Context(), id, ct.UpdateProductJSONRequestBody{Name: name})
				if err != nil {
					return nameRaceOutcome{err: err}
				}
				return nameRaceOutcome{status: resp.StatusCode(), conflict: resp.JSON409}
			}))
	})

	t.Run("Workspace rename", func(t *testing.T) {
		t.Parallel()
		product := createTestProductContext(t)
		org := product.CreateOrganization(t, "Org-"+uuid.NewString(), nil)
		client, _ := product.CreateAPIKeyClientWithScopes([]string{"workspace:create", "workspace:update"})
		assertOneWinnerPerName(t, http.StatusOK, "WORKSPACE_NAME_DUPLICATE",
			renameToSameName(t, func(initialName string) string {
				resp, err := client.CreateOrganizationWorkspaceWithResponse(
					t.Context(), product.ProductID, org.Id,
					ct.CreateOrganizationWorkspaceJSONRequestBody{Name: initialName},
				)
				require.NoError(t, err)
				require.Equal(t, http.StatusCreated, resp.StatusCode())
				return resp.JSON201.Id
			}, func(id, name string) nameRaceOutcome {
				resp, err := client.UpdateOrganizationWorkspaceWithResponse(
					t.Context(), product.ProductID, org.Id, id,
					ct.UpdateOrganizationWorkspaceJSONRequestBody{Name: name},
				)
				if err != nil {
					return nameRaceOutcome{err: err}
				}
				return nameRaceOutcome{status: resp.StatusCode(), conflict: resp.JSON409}
			}))
	})
}
