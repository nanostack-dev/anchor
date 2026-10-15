package ct_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/require"

	itshared "anchor/cmd/it/shared"
	itdsl "anchor/cmd/it/shared/dsl"
	domainintegration "anchor/internal/domain/integration"
	"anchor/internal/domain/product/apikey"
)

// cursorFixtureRows is how many rows a cursor fixture creates: seven fill three
// pages of three, the last one short.
const cursorFixtureRows = 7

// Every search pages with an opaque keyset cursor. Each case below builds rows
// that tie on the sort column, walks them three at a time by cursor, and expects
// every row exactly once in the documented order, so a search that orders by a
// non-unique column alone repeats or drops rows at a page boundary.
func TestCursorSearches(t *testing.T) {
	t.Parallel()

	cases := map[string]func(t *testing.T) cursorCase{
		"searchProducts":                       productsCursorCase,
		"searchProductResourcePermissions":     resourcePermissionsCursorCase,
		"searchProductPermissions":             permissionsCursorCase,
		"searchPlatformInvitations":            platformInvitationsCursorCase,
		"searchPlatformUsers":                  platformUsersCursorCase,
		"searchProductRoles":                   rolesCursorCase,
		"searchProductAPIKeys":                 productAPIKeysCursorCase,
		"searchProductUsers":                   productUsersCursorCase,
		"searchOrganizationAPIKeys":            organizationAPIKeysCursorCase,
		"searchProductOrganizations":           organizationsCursorCase,
		"searchOrganizationWorkspaces":         workspacesCursorCase,
		"searchOrganizationMembers":            membersCursorCase,
		"searchOrganizationInvitations":        organizationInvitationsCursorCase,
		"searchProductOrganizationInvitations": productInvitationsCursorCase,
		"searchOrganizationLicenses":           organizationLicensesCursorCase,
		"searchIntegrationAuditLogs":           auditLogsCursorCase,
		"searchUserOrganizations":              userOrganizationsCursorCase,
		"searchEmailTemplates":                 emailTemplatesCursorCase,
		"searchEmailSends":                     emailSendsCursorCase,
		"searchLicenseTemplates":               licenseTemplatesCursorCase,
		"searchOrganizationLicenseHistory":     licenseHistoryCursorCase,
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			build(t).run(t)
		})
	}
}

// timeOrdered describes rows listed by a time column and an identity column, the
// shape of most searches. Its sorts cover the documented default order and the
// opposite direction named explicitly.
type timeOrdered struct {
	from    string
	idExpr  string
	timeCol string
	where   string
	args    []any
	// field is the sort_by that names timeCol.
	field string
	// defaultDescending is the direction the documented default order runs in.
	defaultDescending bool
}

func (o timeOrdered) order(t *testing.T, descending bool) []string {
	t.Helper()
	dir := direction(descending)
	query := fmt.Sprintf(
		"SELECT %[1]s FROM %[2]s WHERE %[3]s ORDER BY %[4]s %[5]s, %[1]s %[5]s",
		o.idExpr, o.from, o.where, o.timeCol, dir,
	)
	return idsFromSQL(t, query, o.args...)
}

func (o timeOrdered) sorts() []cursorSort {
	defaultOrder := cursorSort{
		name:       "the default order",
		descending: o.defaultDescending,
		order:      func(t *testing.T) []string { return o.order(t, o.defaultDescending) },
	}
	opposite := cursorSort{
		name:       "the opposite direction",
		sortBy:     o.field,
		descending: !o.defaultDescending,
		order:      func(t *testing.T) []string { return o.order(t, !o.defaultDescending) },
	}
	return []cursorSort{defaultOrder, opposite}
}

func httpCall[R interface {
	StatusCode() int
}](response R, err error, body func(R) []byte) (int, []byte, error) {
	if err != nil {
		return 0, nil, err
	}
	return response.StatusCode(), body(response), nil
}

func productsCursorCase(t *testing.T) cursorCase {
	tenant := itdsl.Given(t).Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true}).Build().Tenant("t")
	for range cursorFixtureRows {
		response, err := tenant.OwnerClient.CreateProductWithResponse(
			t.Context(), ct.ProductRequest{Name: "product-" + ids.MustNew("ct")},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, response.StatusCode(), string(response.Body))
	}
	execSQL(t, "UPDATE products SET created_at = $1 WHERE platform_tenant_id = $2", tiedAt(), tenant.ID)

	ordered := timeOrdered{
		from: "products", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "platform_tenant_id = $1", args: []any{tenant.ID},
	}
	byName := cursorSort{
		name: "by name", sortBy: "name",
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM products WHERE platform_tenant_id = $1 ORDER BY name ASC, id ASC",
				tenant.ID,
			)
		},
	}
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := tenant.OwnerClient.SearchProductsWithBodyWithResponse(ctx, "application/json", body)
			return httpCall(response, err, func(r *ct.SearchProductsResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), byName),
	}
}

func resourcePermissionsCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	names := make([]string, cursorFixtureRows)
	for i := range names {
		names[i] = fmt.Sprintf("res%d:read", i)
	}
	product.CreateProductResourcePermissions(t, names...)
	execSQL(t, "UPDATE product_resource_permissions SET created_at = $1 WHERE product_id = $2",
		tiedAt(), product.ProductID)

	ordered := timeOrdered{
		from: "product_resource_permissions", idExpr: "name", timeCol: "created_at", field: "created_at",
		where: "product_id = $1", args: []any{product.ProductID},
	}
	client := product.OwnerAuthenticatedClient()
	return cursorCase{
		field: "created_at",
		idOf:  func(item map[string]any) string { return fmt.Sprint(item["name"]) },
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductResourcePermissionsWithBodyWithResponse(
				ctx, product.ProductID, "application/json", body,
			)
			return httpCall(
				response,
				err,
				func(r *ct.SearchProductResourcePermissionsResponse) []byte { return r.Body },
			)
		},
		sorts: ordered.sorts(),
	}
}

func permissionsCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	execSQL(t, "UPDATE product_permissions SET created_at = $1 WHERE product_id = $2", tiedAt(), product.ProductID)

	ordered := timeOrdered{
		from: "product_permissions", idExpr: "name", timeCol: "created_at", field: "created_at",
		where: "product_id = $1", args: []any{product.ProductID},
	}
	byName := func(sortBy string) cursorSort {
		return cursorSort{
			name: "sorted by " + sortBy, sortBy: sortBy, descending: true,
			order: func(t *testing.T) []string {
				return idsFromSQL(
					t,
					"SELECT name FROM product_permissions WHERE product_id = $1 ORDER BY name DESC",
					product.ProductID,
				)
			},
		}
	}
	client := product.OwnerAuthenticatedClient()
	return cursorCase{
		field: "created_at",
		idOf:  func(item map[string]any) string { return fmt.Sprint(item["name"]) },
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductPermissionsWithBodyWithResponse(
				ctx, product.ProductID, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchProductPermissionsResponse) []byte { return r.Body })
		},
		// id and resource are documented sort fields that order by the permission name.
		sorts: append(ordered.sorts(), byName("id"), byName("resource"), byName("name")),
	}
}

func platformInvitationsCursorCase(t *testing.T) cursorCase {
	tenant := itdsl.Given(t).Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true}).Build().Tenant("t")
	for range cursorFixtureRows {
		response, err := tenant.OwnerClient.CreatePlatformInvitationWithResponse(
			t.Context(), ct.CreatePlatformInvitationJSONRequestBody{Email: uniqueEmail()},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, response.StatusCode(), string(response.Body))
	}
	execSQL(t, "UPDATE platform_invitations SET created_at = $1 WHERE platform_tenant_id = $2", tiedAt(), tenant.ID)

	ordered := timeOrdered{
		from: "platform_invitations", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "platform_tenant_id = $1", args: []any{tenant.ID},
	}
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := tenant.OwnerClient.SearchPlatformInvitationsWithBodyWithResponse(
				ctx,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchPlatformInvitationsResponse) []byte { return r.Body })
		},
		sorts: ordered.sorts(),
	}
}

func platformUsersCursorCase(t *testing.T) cursorCase {
	builder := itdsl.Given(t).Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true})
	for i := range cursorFixtureRows - 1 {
		builder = builder.PlatformAdmin(itdsl.PlatformAdminOpts{Alias: fmt.Sprintf("admin-%d", i), TenantAlias: "t"})
	}
	tenant := builder.Build().Tenant("t")
	execSQL(t, "UPDATE platform_users SET created_at = $1 WHERE platform_tenant_id = $2", tiedAt(), tenant.ID)

	ordered := timeOrdered{
		from: "platform_users", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "platform_tenant_id = $1", args: []any{tenant.ID},
	}
	tiedRoles := cursorSort{
		name: "sorted by role", sortBy: "role",
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM platform_users WHERE platform_tenant_id = $1 ORDER BY role ASC, id ASC",
				tenant.ID,
			)
		},
	}
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := tenant.OwnerClient.SearchPlatformUsersWithBodyWithResponse(ctx, "application/json", body)
			return httpCall(response, err, func(r *ct.SearchPlatformUsersResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), tiedRoles),
	}
}

func rolesCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	for range cursorFixtureRows {
		createDSLProductRole(t, product, "role-"+ids.MustNew("ct"), nil)
	}
	execSQL(t, "UPDATE product_roles SET created_at = $1 WHERE product_id = $2", tiedAt(), product.ProductID)

	ordered := timeOrdered{
		from: "product_roles", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "product_id = $1", args: []any{product.ProductID},
	}
	byID := cursorSort{
		name: "sorted by id", sortBy: "id", descending: true,
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM product_roles WHERE product_id = $1 ORDER BY id DESC",
				product.ProductID,
			)
		},
	}
	client := product.OwnerAuthenticatedClient()
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductRolesWithBodyWithResponse(
				ctx,
				product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchProductRolesResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), byID),
	}
}

func productAPIKeysCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	for i := range cursorFixtureRows {
		_, _, err := ProductAPIKeySvc.Create(t.Context(), apikey.CreateProductAPIKeyInput{
			ProductID:   product.ProductID,
			Name:        fmt.Sprintf("key-%d-%s", i, itshared.Faker.UUID().V4()),
			Permissions: []string{"product_role:read"},
		})
		require.NoError(t, err)
	}
	execSQL(t, "UPDATE product_api_keys SET created_at = $1 WHERE product_id = $2", tiedAt(), product.ProductID)
	// Half the keys were never used: a nullable sort column with ties on both sides.
	execSQL(t, `UPDATE product_api_keys SET last_used_at = $1
		WHERE id IN (SELECT id FROM product_api_keys WHERE product_id = $2 ORDER BY id LIMIT 4)`,
		tiedAt(), product.ProductID)

	ordered := timeOrdered{
		from: "product_api_keys", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "product_id = $1", args: []any{product.ProductID},
	}
	lastUsed := func(descending bool) cursorSort {
		return cursorSort{
			name: "sorted by last_used_at " + direction(descending), sortBy: "last_used_at", descending: descending,
			order: func(t *testing.T) []string {
				return idsFromSQL(t, fmt.Sprintf(
					"SELECT id FROM product_api_keys WHERE product_id = $1 ORDER BY last_used_at %[1]s, id %[1]s",
					direction(descending)), product.ProductID)
			},
		}
	}
	byID := cursorSort{
		name: "sorted by id", sortBy: "id",
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM product_api_keys WHERE product_id = $1 ORDER BY id ASC",
				product.ProductID,
			)
		},
	}
	client := product.OwnerAuthenticatedClient()
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductAPIKeysWithBodyWithResponse(
				ctx,
				product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchProductAPIKeysResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), lastUsed(false), lastUsed(true), byID),
	}
}

func productUsersCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	for range cursorFixtureRows {
		createDSLProductUser(t, product)
	}
	execSQL(t, "UPDATE product_users SET created_at = $1 WHERE product_id = $2", tiedAt(), product.ProductID)
	// Some users carry no name and the rest share one: a nullable sort column
	// where every non-null value ties.
	execSQL(t, `UPDATE product_users SET name = NULL
		WHERE id IN (SELECT id FROM product_users WHERE product_id = $1 ORDER BY id LIMIT 3)`, product.ProductID)
	execSQL(
		t,
		"UPDATE product_users SET name = 'Same Name' WHERE product_id = $1 AND name IS NOT NULL",
		product.ProductID,
	)

	ordered := timeOrdered{
		from: "product_users", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "product_id = $1", args: []any{product.ProductID},
	}
	byName := func(descending bool) cursorSort {
		return cursorSort{
			name: "sorted by name " + direction(descending), sortBy: "name", descending: descending,
			order: func(t *testing.T) []string {
				return idsFromSQL(t, fmt.Sprintf(
					"SELECT id FROM product_users WHERE product_id = $1 ORDER BY name %[1]s, id %[1]s",
					direction(descending)), product.ProductID)
			},
		}
	}
	client := product.AllScopeAPIKeyClient()
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductUsersWithBodyWithResponse(
				ctx,
				product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchProductUsersResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), byName(false), byName(true)),
	}
}

func organizationAPIKeysCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	organization := product.CreateOrganization(t, "org-"+ids.MustNew("ct"), nil)
	permissions := givenOrganizationAPIKeyResourcePermissions(t, product)
	client := product.AllScopeAPIKeyClient()
	for i := range cursorFixtureRows {
		response, err := client.CreateOrganizationAPIKeyWithResponse(
			t.Context(), product.ProductID, organization.Id,
			ct.CreateOrganizationAPIKeyJSONRequestBody{
				Name:        fmt.Sprintf("key-%d-%s", i, itshared.Faker.UUID().V4()),
				Permissions: []string{permissions.FileRead},
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, response.StatusCode(), string(response.Body))
	}
	execSQL(t, "UPDATE organization_api_keys SET created_at = $1 WHERE organization_id = $2", tiedAt(), organization.Id)

	ordered := timeOrdered{
		from: "organization_api_keys", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "organization_id = $1", args: []any{organization.Id},
	}
	neverUsed := cursorSort{
		name: "sorted by last_used_at, which no key has", sortBy: "last_used_at", descending: true,
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM organization_api_keys WHERE organization_id = $1 ORDER BY last_used_at DESC, id DESC",
				organization.Id,
			)
		},
	}
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationAPIKeysWithBodyWithResponse(
				ctx, product.ProductID, organization.Id, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchOrganizationAPIKeysResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), neverUsed),
	}
}

func organizationsCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	for range cursorFixtureRows {
		product.CreateOrganization(t, "org-"+ids.MustNew("ct"), nil)
	}
	execSQL(t, "UPDATE organizations SET created_at = $1, name = 'Same Name' WHERE product_id = $2",
		tiedAt(), product.ProductID)

	ordered := timeOrdered{
		from: "organizations", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "product_id = $1", args: []any{product.ProductID},
	}
	tiedNames := cursorSort{
		name: "sorted by tied names", sortBy: "name",
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM organizations WHERE product_id = $1 ORDER BY name ASC, id ASC",
				product.ProductID,
			)
		},
	}
	client := product.AllScopeAPIKeyClient()
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductOrganizationsWithBodyWithResponse(
				ctx, product.ProductID, nil, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchProductOrganizationsResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), tiedNames),
	}
}

func workspacesCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	organization := product.CreateOrganization(t, "org-"+ids.MustNew("ct"), nil)
	workspaces := product.Workspaces(organization.Id)
	for range cursorFixtureRows {
		workspaces.Create(ct.ProductWorkspaceRequest{Name: "ws-" + ids.MustNew("ct")})
	}
	execSQL(t, "UPDATE workspaces SET created_at = $1 WHERE organization_id = $2", tiedAt(), organization.Id)

	ordered := timeOrdered{
		from: "workspaces", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "organization_id = $1", args: []any{organization.Id},
	}
	byName := cursorSort{
		name: "sorted by name", sortBy: "name", descending: true,
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM workspaces WHERE organization_id = $1 ORDER BY name DESC, id DESC",
				organization.Id,
			)
		},
	}
	client := product.AllScopeAPIKeyClient()
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationWorkspacesWithBodyWithResponse(
				ctx, product.ProductID, organization.Id, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchOrganizationWorkspacesResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), byName),
	}
}

func membersCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	organization := product.CreateOrganization(t, "org-"+ids.MustNew("ct"), nil)
	role := createDSLProductRole(t, product, "role-"+ids.MustNew("ct"), nil)
	for range cursorFixtureRows {
		member := createDSLProductUser(t, product)
		createDSLMembership(t, product, member.ID, organization.Id, role.ID)
	}
	execSQL(
		t,
		"UPDATE organization_memberships SET created_at = $1 WHERE organization_id = $2",
		tiedAt(),
		organization.Id,
	)

	// The default is the newest member first. Before keyset pagination this search
	// had no ORDER BY at all, so its pages overlapped and dropped members.
	ordered := timeOrdered{
		from: "organization_memberships", idExpr: "product_user_id", timeCol: "created_at", field: "joined_at",
		where: "organization_id = $1", args: []any{organization.Id}, defaultDescending: true,
	}
	byEmail := func(descending bool) cursorSort {
		return cursorSort{
			name: "sorted by email " + direction(descending), sortBy: "email", descending: descending,
			order: func(t *testing.T) []string {
				return idsFromSQL(t, fmt.Sprintf(`SELECT m.product_user_id FROM organization_memberships m
					JOIN product_users u ON u.id = m.product_user_id
					WHERE m.organization_id = $1 ORDER BY u.email %[1]s, m.product_user_id %[1]s`,
					direction(descending)), organization.Id)
			},
		}
	}
	client := product.AllScopeAPIKeyClient()
	return cursorCase{
		field: "joined_at",
		idOf:  func(item map[string]any) string { return fmt.Sprint(item["product_user_id"]) },
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationMembersWithBodyWithResponse(
				ctx, product.ProductID, organization.Id, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchOrganizationMembersResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), byEmail(false), byEmail(true)),
	}
}

func organizationInvitationsCursorCase(t *testing.T) cursorCase {
	w := newWorld(t)
	for range cursorFixtureRows {
		w.invite(uniqueEmail())
	}
	execSQL(
		t,
		"UPDATE organization_invitations SET created_at = $1 WHERE organization_id = $2",
		tiedAt(),
		w.organizationID,
	)

	ordered := timeOrdered{
		from: "organization_invitations", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "organization_id = $1", args: []any{w.organizationID},
	}
	tiedExpiry := cursorSort{
		name: "sorted by expires_at", sortBy: "expires_at", descending: true,
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM organization_invitations WHERE organization_id = $1 ORDER BY expires_at DESC, id DESC",
				w.organizationID,
			)
		},
	}
	client := w.product.AllScopeAPIKeyClient()
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationInvitationsWithBodyWithResponse(
				ctx, w.product.ProductID, w.organizationID, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchOrganizationInvitationsResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), tiedExpiry),
	}
}

func productInvitationsCursorCase(t *testing.T) cursorCase {
	w := newWorld(t)
	second := w.newOrganization()
	for i := range cursorFixtureRows {
		if i%2 == 0 {
			w.invite(uniqueEmail())
			continue
		}
		w.invitations.Create(second, ct.CreateOrganizationInvitationJSONRequestBody{
			Email:  openapi_types.Email(uniqueEmail()),
			RoleId: w.roleID,
		})
	}
	execSQL(
		t,
		"UPDATE organization_invitations SET created_at = $1 WHERE product_id = $2",
		tiedAt(),
		w.product.ProductID,
	)

	ordered := timeOrdered{
		from: "organization_invitations", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "product_id = $1", args: []any{w.product.ProductID},
	}
	client := w.product.AllScopeAPIKeyClient()
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductOrganizationInvitationsWithBodyWithResponse(
				ctx, w.product.ProductID, "application/json", body,
			)
			return httpCall(
				response,
				err,
				func(r *ct.SearchProductOrganizationInvitationsResponse) []byte { return r.Body },
			)
		},
		sorts: ordered.sorts(),
	}
}

func organizationLicensesCursorCase(t *testing.T) cursorCase {
	w := newLicenseWorld(t)
	organizations := []string{w.OrganizationID()}
	for range cursorFixtureRows - 1 {
		organizations = append(organizations, w.NewOrganization())
	}
	// Three organizations hold a license and four hold none: the instantiated_at
	// column is NULL for four rows and ties for the other three.
	for _, organizationID := range organizations[:3] {
		w.License().For(organizationID).Instantiate(w.TemplateID())
	}
	execSQL(t, "UPDATE organizations SET name = 'Same Name' WHERE product_id = $1", w.productID())
	execSQL(t, "UPDATE organization_licenses SET instantiated_at = $1 WHERE product_id = $2", tiedAt(), w.productID())

	byName := func(descending bool) cursorSort {
		return cursorSort{
			name: "sorted by organization_name " + direction(
				descending,
			),
			sortBy:     "organization_name",
			descending: descending,
			order: func(t *testing.T) []string {
				return idsFromSQL(t, fmt.Sprintf(
					"SELECT id FROM organizations WHERE product_id = $1 ORDER BY name %[1]s, id %[1]s",
					direction(descending),
				),
					w.productID())
			},
		}
	}
	byInstantiatedAt := func(descending bool) cursorSort {
		return cursorSort{
			name: "sorted by instantiated_at " + direction(
				descending,
			),
			sortBy:     "instantiated_at",
			descending: descending,
			order: func(t *testing.T) []string {
				return idsFromSQL(t, fmt.Sprintf(`SELECT o.id FROM organizations o
					LEFT JOIN organization_licenses l ON l.organization_id = o.id
					WHERE o.product_id = $1 ORDER BY l.instantiated_at %[1]s NULLS LAST, o.id %[1]s`, direction(descending)),
					w.productID())
			},
		}
	}
	defaultOrder := byName(false)
	defaultOrder.name = "the default order"
	defaultOrder.sortBy = ""

	client := w.client()
	return cursorCase{
		field: "organization_name",
		idOf:  func(item map[string]any) string { return fmt.Sprint(item["organization_id"]) },
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationLicensesWithBodyWithResponse(
				ctx,
				w.productID(),
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchOrganizationLicensesResponse) []byte { return r.Body })
		},
		sorts: []cursorSort{defaultOrder, byName(true), byInstantiatedAt(false), byInstantiatedAt(true)},
	}
}

func auditLogsCursorCase(t *testing.T) cursorCase {
	tc := newEmailTestCtx(t)
	instance := domainintegration.Instance{
		PlatformTenantID: tc.tenantID,
		ProductID:        tc.product.ProductID,
		ProviderType:     domainintegration.ProviderTypeSMTP,
		ConfigJSON:       []byte(`{}`),
		ConfigVersion:    1,
		IsEnabled:        true,
		Status:           domainintegration.StatusActive,
	}
	instance.GenerateID()
	created, createErr := IntegrationRepo.Create(t.Context(), instance)
	require.NoError(t, createErr)

	for i := range cursorFixtureRows {
		severity := "INFO"
		if i%2 == 0 {
			severity = "ERROR"
		}
		execSQL(t, `INSERT INTO integration_audit_logs
			(id, integration_instance_id, action, entity_type, severity, message, created_at)
			VALUES ($1, $2, 'SYNC', 'user', $3, 'synced', $4)`,
			ids.MustNew("ial"), created.ID, severity, tiedAt())
	}

	ordered := timeOrdered{
		from: "integration_audit_logs", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "integration_instance_id = $1", args: []any{created.ID}, defaultDescending: true,
	}
	bySeverity := cursorSort{
		name: "sorted by severity", sortBy: "severity", descending: true,
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM integration_audit_logs WHERE integration_instance_id = $1 ORDER BY severity DESC, id DESC",
				created.ID,
			)
		},
	}
	byTiedAction := cursorSort{
		name: "sorted by tied action", sortBy: "action",
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM integration_audit_logs WHERE integration_instance_id = $1 ORDER BY action ASC, id ASC",
				created.ID,
			)
		},
	}
	client := tc.product.OwnerAuthenticatedClient()
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchIntegrationAuditLogsWithBodyWithResponse(
				ctx, tc.product.ProductID, created.ID, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchIntegrationAuditLogsResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), bySeverity, byTiedAction),
	}
}

func userOrganizationsCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	member := createDSLProductUser(t, product)
	role := createDSLProductRole(t, product, "role-"+ids.MustNew("ct"), nil)
	for range cursorFixtureRows {
		organization := product.CreateOrganization(t, "org-"+ids.MustNew("ct"), nil)
		createDSLMembership(t, product, member.ID, organization.Id, role.ID)
	}
	execSQL(t, "UPDATE organization_memberships SET created_at = $1 WHERE product_user_id = $2", tiedAt(), member.ID)
	execSQL(t, `UPDATE organizations SET name = 'Same Name'
		WHERE id IN (SELECT organization_id FROM organization_memberships WHERE product_user_id = $1)`, member.ID)

	ordered := timeOrdered{
		from: "organization_memberships", idExpr: "organization_id", timeCol: "created_at", field: "joined_at",
		where: "product_user_id = $1", args: []any{member.ID},
	}
	byTiedName := cursorSort{
		name: "sorted by tied name", sortBy: "name", descending: true,
		order: func(t *testing.T) []string {
			return idsFromSQL(t, `SELECT m.organization_id FROM organization_memberships m
				JOIN organizations o ON o.id = m.organization_id
				WHERE m.product_user_id = $1 ORDER BY o.name DESC, m.organization_id DESC`, member.ID)
		},
	}
	client := product.AllScopeAPIKeyClient()
	return cursorCase{
		field: "joined_at",
		idOf: func(item map[string]any) string {
			organization, _ := item["organization"].(map[string]any)
			return fmt.Sprint(organization["id"])
		},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchUserOrganizationsWithBodyWithResponse(
				ctx, product.ProductID, member.ID, nil, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchUserOrganizationsResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), byTiedName),
	}
}

func emailTemplatesCursorCase(t *testing.T) cursorCase {
	tc := newEmailTestCtx(t)
	client := tc.product.OwnerAuthenticatedClient()
	for range cursorFixtureRows {
		response, err := client.CreateEmailTemplateWithResponse(
			t.Context(), tc.product.ProductID,
			ct.CreateEmailTemplateJSONRequestBody{
				Slug: uniqueSlug(), Name: "Same Name", Subject: "Subject", BodyHtml: "<p>Body</p>",
			},
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, response.StatusCode(), string(response.Body))
	}
	execSQL(t, "UPDATE email_templates SET created_at = $1 WHERE product_id = $2", tiedAt(), tc.product.ProductID)

	ordered := timeOrdered{
		from: "email_templates", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "product_id = $1", args: []any{tc.product.ProductID}, defaultDescending: true,
	}
	tiedNames := cursorSort{
		name: "sorted by tied name", sortBy: "name",
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM email_templates WHERE product_id = $1 ORDER BY name ASC, id ASC",
				tc.product.ProductID,
			)
		},
	}
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchEmailTemplatesWithBodyWithResponse(
				ctx,
				tc.product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchEmailTemplatesResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), tiedNames),
	}
}

func emailSendsCursorCase(t *testing.T) cursorCase {
	tc := newEmailTestCtx(t)
	for i := range cursorFixtureRows {
		status := "SENT"
		if i%2 == 0 {
			status = "FAILED"
		}
		execSQL(t, `INSERT INTO email_send_records
			(id, platform_tenant_id, product_id, to_address, from_address, subject, body_html, status, created_at)
			VALUES ($1, $2, $3, $4, 'noreply@tryanchor.dev', 'Subject', '<p>Body</p>', $5, $6)`,
			ids.MustNew("esnd"), tc.tenantID, tc.product.ProductID, uniqueEmail(), status, tiedAt())
	}

	ordered := timeOrdered{
		from: "email_send_records", idExpr: "id", timeCol: "created_at", field: "created_at",
		where: "product_id = $1", args: []any{tc.product.ProductID}, defaultDescending: true,
	}
	byStatus := cursorSort{
		name: "sorted by status", sortBy: "status",
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM email_send_records WHERE product_id = $1 ORDER BY status ASC, id ASC",
				tc.product.ProductID,
			)
		},
	}
	client := tc.product.OwnerAuthenticatedClient()
	return cursorCase{
		field: "created_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchEmailSendsWithBodyWithResponse(
				ctx,
				tc.product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchEmailSendsResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), byStatus),
	}
}

func licenseTemplatesCursorCase(t *testing.T) cursorCase {
	tc := newTemplateCtx(t)
	// An archived template frees its name, so seven templates can share one.
	for i := range cursorFixtureRows {
		template := createTemplate(t, tc, "Tier", validTemplateValues())
		if i < cursorFixtureRows-1 {
			response := archiveTemplate(t, tc, template.Id)
			require.Equal(t, http.StatusOK, response.StatusCode(), string(response.Body))
		}
	}
	execSQL(t, "UPDATE license_templates SET created_at = $1 WHERE product_id = $2", tiedAt(), tc.product.ProductID)

	ordered := timeOrdered{
		from: "license_templates", idExpr: "id", timeCol: "name", field: "name",
		where: "product_id = $1", args: []any{tc.product.ProductID},
	}
	byStatus := cursorSort{
		name: "sorted by status", sortBy: "status",
		order: func(t *testing.T) []string {
			return idsFromSQL(t, "SELECT id FROM license_templates WHERE product_id = $1 ORDER BY status ASC, id ASC",
				tc.product.ProductID)
		},
	}
	byCreatedAt := cursorSort{
		name: "sorted by tied created_at", sortBy: "created_at", descending: true,
		order: func(t *testing.T) []string {
			return idsFromSQL(
				t,
				"SELECT id FROM license_templates WHERE product_id = $1 ORDER BY created_at DESC, id DESC",
				tc.product.ProductID,
			)
		},
	}
	client := tc.product.OwnerAuthenticatedClient()
	return cursorCase{
		field: "name",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchLicenseTemplatesWithBodyWithResponse(
				ctx,
				tc.product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchLicenseTemplatesResponse) []byte { return r.Body })
		},
		sorts: append(ordered.sorts(), byStatus, byCreatedAt),
	}
}

func licenseHistoryCursorCase(t *testing.T) cursorCase {
	w := newLicensedWorld(t)
	// One adjustment writes one entry per moved field, all at one changed_at.
	w.License().Adjust(ct.LicenseTemplateValues{"flows": 1, "sso": false, "support_tier": "basic", "region": "us-east"})
	w.License().
		Adjust(ct.LicenseTemplateValues{"flows": 2, "sso": true, "support_tier": "priority", "region": "ca-central"})
	execSQL(t, "UPDATE organization_license_changes SET changed_at = $1 WHERE organization_id = $2",
		tiedAt(), w.OrganizationID())

	ordered := timeOrdered{
		from: "organization_license_changes", idExpr: "id", timeCol: "changed_at", field: "changed_at",
		where: "organization_id = $1", args: []any{w.OrganizationID()}, defaultDescending: true,
	}
	client := w.client()
	return cursorCase{
		field: "changed_at",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationLicenseHistoryWithBodyWithResponse(
				ctx, w.productID(), w.OrganizationID(), "application/json", body,
			)
			return httpCall(
				response,
				err,
				func(r *ct.SearchOrganizationLicenseHistoryResponse) []byte { return r.Body },
			)
		},
		sorts: ordered.sorts(),
	}
}
