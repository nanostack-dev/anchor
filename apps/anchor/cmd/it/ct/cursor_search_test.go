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
// that tie on the sort column and hands the search to the searchtest conformance
// kit, which walks them three at a time by cursor in the default order and in
// every sort field both ways, and expects every row exactly once in the order
// SQL gives. A search that orders by a non-unique column alone repeats or drops
// rows at a page boundary. The kit also refuses malformed, oversized, forged and
// foreign cursors and an unknown sort_by in every scope, empty ones included.
func TestCursorSearches(t *testing.T) {
	// Every resource search walks tied rows exactly once in each sort order and refuses invalid cursors.
	t.Parallel()

	cases := map[string]func(t *testing.T) cursorCase{
		// The products search obeys the shared ordering and cursor-refusal contract.
		"searchProducts": productsCursorCase,
		// The product resource permissions search obeys the shared ordering and cursor-refusal contract.
		"searchProductResourcePermissions": resourcePermissionsCursorCase,
		// The product permissions search obeys the shared ordering and cursor-refusal contract.
		"searchProductPermissions": permissionsCursorCase,
		// The platform invitations search obeys the shared ordering and cursor-refusal contract.
		"searchPlatformInvitations": platformInvitationsCursorCase,
		// The platform users search obeys the shared ordering and cursor-refusal contract.
		"searchPlatformUsers": platformUsersCursorCase,
		// The product roles search obeys the shared ordering and cursor-refusal contract.
		"searchProductRoles": rolesCursorCase,
		// The product API keys search obeys the shared ordering and cursor-refusal contract.
		"searchProductAPIKeys": productAPIKeysCursorCase,
		// The product users search obeys the shared ordering and cursor-refusal contract.
		"searchProductUsers": productUsersCursorCase,
		// The organization API keys search obeys the shared ordering and cursor-refusal contract.
		"searchOrganizationAPIKeys": organizationAPIKeysCursorCase,
		// The product organizations search obeys the shared ordering and cursor-refusal contract.
		"searchProductOrganizations": organizationsCursorCase,
		// The organization workspaces search obeys the shared ordering and cursor-refusal contract.
		"searchOrganizationWorkspaces": workspacesCursorCase,
		// The organization members search obeys the shared ordering and cursor-refusal contract.
		"searchOrganizationMembers": membersCursorCase,
		// The organization invitations search obeys the shared ordering and cursor-refusal contract.
		"searchOrganizationInvitations": organizationInvitationsCursorCase,
		// The product organization invitations search obeys the shared ordering and cursor-refusal contract.
		"searchProductOrganizationInvitations": productInvitationsCursorCase,
		// The organization licenses search obeys the shared ordering and cursor-refusal contract.
		"searchOrganizationLicenses": organizationLicensesCursorCase,
		// The integration audit logs search obeys the shared ordering and cursor-refusal contract.
		"searchIntegrationAuditLogs": auditLogsCursorCase,
		// The user organizations search obeys the shared ordering and cursor-refusal contract.
		"searchUserOrganizations": userOrganizationsCursorCase,
		// The email templates search obeys the shared ordering and cursor-refusal contract.
		"searchEmailTemplates": emailTemplatesCursorCase,
		// The email sends search obeys the shared ordering and cursor-refusal contract.
		"searchEmailSends": emailSendsCursorCase,
		// The license templates search obeys the shared ordering and cursor-refusal contract.
		"searchLicenseTemplates": licenseTemplatesCursorCase,
		// The organization license history search obeys the shared ordering and cursor-refusal contract.
		"searchOrganizationLicenseHistory": licenseHistoryCursorCase,
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			// Each resource search walks every tied row once and enforces the cursor contract.
			t.Parallel()
			build(t).run(t, name)
		})
	}
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

	return cursorCase{
		rows: orderedRows{
			from: "products", where: "platform_tenant_id = $1", args: []any{tenant.ID}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "name", expr: "name"},
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"ids": []string{"no-such-id"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := tenant.OwnerClient.SearchProductsWithBodyWithResponse(ctx, "application/json", body)
			return httpCall(response, err, func(r *ct.SearchProductsResponse) []byte { return r.Body })
		},
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

	client := product.OwnerAuthenticatedClient()
	return cursorCase{
		rows: orderedRows{
			from:  "product_resource_permissions",
			where: "product_id = $1", args: []any{product.ProductID}, idExpr: "name",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "name", expr: "name"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"names": []string{"nothing:read"}}},
		idField: "name",
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
	}
}

func permissionsCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	execSQL(t, "UPDATE product_permissions SET created_at = $1 WHERE product_id = $2", tiedAt(), product.ProductID)

	client := product.OwnerAuthenticatedClient()
	return cursorCase{
		rows: orderedRows{
			from: "product_permissions", where: "product_id = $1", args: []any{product.ProductID}, idExpr: "name",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "name", expr: "name"},
				{name: "id", expr: "name"},
				{name: "resource", expr: "name"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"names": []string{"nothing:read"}}},
		idField: "name",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductPermissionsWithBodyWithResponse(
				ctx, product.ProductID, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchProductPermissionsResponse) []byte { return r.Body })
		},
		// id and resource are documented sort fields that order by the permission name.
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

	return cursorCase{
		rows: orderedRows{
			from: "platform_invitations", where: "platform_tenant_id = $1", args: []any{tenant.ID}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "email", expr: "email"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"ids": []string{"no-such-id"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := tenant.OwnerClient.SearchPlatformInvitationsWithBodyWithResponse(
				ctx,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchPlatformInvitationsResponse) []byte { return r.Body })
		},
	}
}

func platformUsersCursorCase(t *testing.T) cursorCase {
	builder := itdsl.Given(t).Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true})
	for i := range cursorFixtureRows - 1 {
		builder = builder.PlatformAdmin(itdsl.PlatformAdminOpts{Alias: fmt.Sprintf("admin-%d", i), TenantAlias: "t"})
	}
	tenant := builder.Build().Tenant("t")
	execSQL(t, "UPDATE platform_users SET created_at = $1 WHERE platform_tenant_id = $2", tiedAt(), tenant.ID)

	return cursorCase{
		rows: orderedRows{
			from: "platform_users", where: "platform_tenant_id = $1", args: []any{tenant.ID}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "email", expr: "email"},
				{name: "role", expr: "role"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"ids": []string{"no-such-id"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := tenant.OwnerClient.SearchPlatformUsersWithBodyWithResponse(ctx, "application/json", body)
			return httpCall(response, err, func(r *ct.SearchPlatformUsersResponse) []byte { return r.Body })
		},
	}
}

func rolesCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	for range cursorFixtureRows {
		createDSLProductRole(t, product, "role-"+ids.MustNew("ct"), nil)
	}
	execSQL(t, "UPDATE product_roles SET created_at = $1 WHERE product_id = $2", tiedAt(), product.ProductID)

	client := product.OwnerAuthenticatedClient()
	return cursorCase{
		rows: orderedRows{
			from: "product_roles", where: "product_id = $1", args: []any{product.ProductID}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "name", expr: "name"},
				{name: "id", expr: "id"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"ids": []string{"no-such-id"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductRolesWithBodyWithResponse(
				ctx,
				product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchProductRolesResponse) []byte { return r.Body })
		},
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

	client := product.OwnerAuthenticatedClient()
	return cursorCase{
		rows: orderedRows{
			from: "product_api_keys", where: "product_id = $1", args: []any{product.ProductID}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "name", expr: "name"},
				{name: "id", expr: "id"},
				{name: "status", expr: "status"},
				{name: "last_used_at", expr: "last_used_at"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"ids": []string{"no-such-id"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductAPIKeysWithBodyWithResponse(
				ctx,
				product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchProductAPIKeysResponse) []byte { return r.Body })
		},
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

	client := product.AllScopeAPIKeyClient()
	return cursorCase{
		rows: orderedRows{
			from: "product_users", where: "product_id = $1", args: []any{product.ProductID}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "email", expr: "email"},
				{name: "name", expr: "name"},
				{name: "status", expr: "status"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"ids": []string{"no-such-id"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductUsersWithBodyWithResponse(
				ctx,
				product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchProductUsersResponse) []byte { return r.Body })
		},
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

	return cursorCase{
		rows: orderedRows{
			from: "organization_api_keys", where: "organization_id = $1", args: []any{organization.Id}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "name", expr: "name"},
				{name: "id", expr: "id"},
				{name: "status", expr: "status"},
				{name: "last_used_at", expr: "last_used_at"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"ids": []string{"no-such-id"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationAPIKeysWithBodyWithResponse(
				ctx, product.ProductID, organization.Id, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchOrganizationAPIKeysResponse) []byte { return r.Body })
		},
	}
}

func organizationsCursorCase(t *testing.T) cursorCase {
	product := createTestProductContext(t)
	for range cursorFixtureRows {
		product.CreateOrganization(t, "org-"+ids.MustNew("ct"), nil)
	}
	execSQL(t, "UPDATE organizations SET created_at = $1, name = 'Same Name' WHERE product_id = $2",
		tiedAt(), product.ProductID)

	client := product.AllScopeAPIKeyClient()
	return cursorCase{
		rows: orderedRows{
			from: "organizations", where: "product_id = $1", args: []any{product.ProductID}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "name", expr: "name"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"ids": []string{"no-such-id"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchProductOrganizationsWithBodyWithResponse(
				ctx, product.ProductID, nil, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchProductOrganizationsResponse) []byte { return r.Body })
		},
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

	client := product.AllScopeAPIKeyClient()
	return cursorCase{
		rows: orderedRows{
			from: "workspaces", where: "organization_id = $1", args: []any{organization.Id}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "name", expr: "name"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"ids": []string{"no-such-id"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationWorkspacesWithBodyWithResponse(
				ctx, product.ProductID, organization.Id, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchOrganizationWorkspacesResponse) []byte { return r.Body })
		},
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
	client := product.AllScopeAPIKeyClient()
	return cursorCase{
		rows: orderedRows{
			from:  "organization_memberships m JOIN product_users u ON u.id = m.product_user_id",
			where: "m.organization_id = $1", args: []any{organization.Id}, idExpr: "m.product_user_id",
			defaultField: "joined_at", defaultDescending: true,
			fields: []sortField{
				{name: "joined_at", expr: "m.created_at"},
				{name: "email", expr: "u.email"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"product_user_ids": []string{"no-such-id"}}},
		idField: "product_user_id",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationMembersWithBodyWithResponse(
				ctx, product.ProductID, organization.Id, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchOrganizationMembersResponse) []byte { return r.Body })
		},
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

	client := w.product.AllScopeAPIKeyClient()
	return cursorCase{
		rows: orderedRows{
			from:  "organization_invitations",
			where: "organization_id = $1", args: []any{w.organizationID}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "email", expr: "email"},
				{name: "expires_at", expr: "expires_at"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"emails": []string{"nobody@invited.test"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationInvitationsWithBodyWithResponse(
				ctx, w.product.ProductID, w.organizationID, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchOrganizationInvitationsResponse) []byte { return r.Body })
		},
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

	client := w.product.AllScopeAPIKeyClient()
	return cursorCase{
		rows: orderedRows{
			from: "organization_invitations", where: "product_id = $1", args: []any{w.product.ProductID}, idExpr: "id",
			defaultField: "created_at",
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "email", expr: "email"},
				{name: "expires_at", expr: "expires_at"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"emails": []string{"nobody@invited.test"}}},
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

	client := w.client()
	return cursorCase{
		rows: orderedRows{
			from:  "organizations o LEFT JOIN organization_licenses l ON l.organization_id = o.id",
			where: "o.product_id = $1", args: []any{w.productID()}, idExpr: "o.id",
			defaultField: "organization_name",
			fields: []sortField{
				{name: "organization_name", expr: "o.name"},
				{name: "instantiated_at", expr: "l.instantiated_at", nulls: "NULLS LAST"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"organization_ids": []string{"no-such-id"}}},
		idField: "organization_id",
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchOrganizationLicensesWithBodyWithResponse(
				ctx,
				w.productID(),
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchOrganizationLicensesResponse) []byte { return r.Body })
		},
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

	client := tc.product.OwnerAuthenticatedClient()
	return cursorCase{
		rows: orderedRows{
			from:              "integration_audit_logs",
			where:             "integration_instance_id = $1",
			args:              []any{created.ID},
			idExpr:            "id",
			defaultField:      "created_at",
			defaultDescending: true,
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "action", expr: "action"},
				{name: "severity", expr: "severity"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"created_before": "2000-01-01T00:00:00Z"}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchIntegrationAuditLogsWithBodyWithResponse(
				ctx, tc.product.ProductID, created.ID, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchIntegrationAuditLogsResponse) []byte { return r.Body })
		},
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

	client := product.AllScopeAPIKeyClient()
	return cursorCase{
		rows: orderedRows{
			from:  "organization_memberships m JOIN organizations o ON o.id = m.organization_id",
			where: "m.product_user_id = $1", args: []any{member.ID}, idExpr: "m.organization_id",
			defaultField: "joined_at",
			fields: []sortField{
				{name: "joined_at", expr: "m.created_at"},
				{name: "name", expr: "o.name"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"organization_ids": []string{"no-such-id"}}},
		decode:  nestedIDDecoder("organization"),
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchUserOrganizationsWithBodyWithResponse(
				ctx, product.ProductID, member.ID, nil, "application/json", body,
			)
			return httpCall(response, err, func(r *ct.SearchUserOrganizationsResponse) []byte { return r.Body })
		},
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

	return cursorCase{
		rows: orderedRows{
			from: "email_templates", where: "product_id = $1", args: []any{tc.product.ProductID}, idExpr: "id",
			defaultField: "created_at", defaultDescending: true,
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
				{name: "name", expr: "name"},
				{name: "slug", expr: "slug"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"slugs": []string{"no-such-slug"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchEmailTemplatesWithBodyWithResponse(
				ctx,
				tc.product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchEmailTemplatesResponse) []byte { return r.Body })
		},
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

	client := tc.product.OwnerAuthenticatedClient()
	return cursorCase{
		rows: orderedRows{
			from: "email_send_records", where: "product_id = $1", args: []any{tc.product.ProductID}, idExpr: "id",
			defaultField: "created_at", defaultDescending: true,
			fields: []sortField{
				{name: "created_at", expr: "created_at"},
				{name: "status", expr: "status"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"template_ids": []string{"no-such-id"}}},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchEmailSendsWithBodyWithResponse(
				ctx,
				tc.product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchEmailSendsResponse) []byte { return r.Body })
		},
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

	client := tc.product.OwnerAuthenticatedClient()
	return cursorCase{
		rows: orderedRows{
			from: "license_templates", where: "product_id = $1", args: []any{tc.product.ProductID}, idExpr: "id",
			defaultField: "name",
			fields: []sortField{
				{name: "name", expr: "name"},
				{name: "status", expr: "status"},
				{name: "created_at", expr: "created_at"},
				{name: "updated_at", expr: "updated_at"},
			},
		},
		noMatch: map[string]any{"full_text_search": "no-such-template-zzz"},
		call: func(ctx context.Context, body io.Reader) (int, []byte, error) {
			response, err := client.SearchLicenseTemplatesWithBodyWithResponse(
				ctx,
				tc.product.ProductID,
				"application/json",
				body,
			)
			return httpCall(response, err, func(r *ct.SearchLicenseTemplatesResponse) []byte { return r.Body })
		},
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

	client := w.client()
	return cursorCase{
		rows: orderedRows{
			from:              "organization_license_changes",
			where:             "organization_id = $1",
			args:              []any{w.OrganizationID()},
			idExpr:            "id",
			defaultField:      "changed_at",
			defaultDescending: true,
			fields: []sortField{
				{name: "changed_at", expr: "changed_at"},
			},
		},
		noMatch: map[string]any{"filter": map[string]any{"changed_before": "2000-01-01T00:00:00Z"}},
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
	}
}
