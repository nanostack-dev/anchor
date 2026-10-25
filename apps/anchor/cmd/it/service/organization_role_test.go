package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/organizationrole"
	"anchor/internal/domain/product/role"
	orgroleservice "anchor/internal/organizationrole/service"
)

// A release before keys wrote role templates with none. Only the startup
// backfill keys them, and no route reaches it.
func TestRoleTemplateKeyBackfill(t *testing.T) {
	t.Run("A template written without a key gets its name in lower case", func(t *testing.T) {
		ctx := t.Context()
		productID := GivenATenantAndProduct(t).Product.ID
		template := givenUnkeyedTemplate(t, productID, "Legacy Admin "+uuid.NewString())

		require.NoError(
			t,
			orgroleservice.BackfillCopies(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		assert.Equal(t, strings.ToLower(template.Name), templateKey(t, template.ID))
	})

	t.Run("A template whose name another template holds as its key gets its own ID", func(t *testing.T) {
		ctx := t.Context()
		productID := GivenATenantAndProduct(t).Product.ID
		name := "Viewer " + uuid.NewString()
		keyed := role.ProductRole{ProductID: productID, Name: "Reader " + uuid.NewString(), Key: strings.ToLower(name)}
		keyed.GenerateID()
		_, err := ProductRoleRepo.Create(ctx, keyed)
		require.NoError(t, err)
		unkeyed := givenUnkeyedTemplate(t, productID, name)

		require.NoError(
			t,
			orgroleservice.BackfillCopies(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		assert.Equal(t, unkeyed.ID, templateKey(t, unkeyed.ID))
	})

	t.Run("Running the backfill again changes nothing", func(t *testing.T) {
		ctx := t.Context()
		productID := GivenATenantAndProduct(t).Product.ID
		template := givenUnkeyedTemplate(t, productID, "Admin "+uuid.NewString())
		require.NoError(
			t,
			orgroleservice.BackfillCopies(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		require.NoError(
			t,
			orgroleservice.BackfillCopies(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		assert.Equal(t, strings.ToLower(template.Name), templateKey(t, template.ID))
	})
}

// Template routes and writes treat an Organization's copy of a template as
// part of the template. The release before copies were made already did, so
// it ran beside the release that makes them.
func TestOrganizationCopiesWrittenByTheNextRelease(t *testing.T) {
	t.Run("Template searches leave the copies out", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		found, err := ProductRoleRepo.SearchByProductID(t.Context(), fixture.productID,
			search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole]{
				Pagination: search.Pagination{Limit: 10},
			})

		require.NoError(t, err)
		require.Len(t, found.Items, 1)
		assert.Equal(t, fixture.templateID, found.Items[0].ID)
	})

	t.Run("A member on a copy shows the template's ID", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		member, err := MembershipService.GetMember(t.Context(), organization.GetMemberInput{
			ProductID: fixture.productID, OrganizationID: fixture.organizationID, ProductUserID: fixture.userID,
		})

		require.NoError(t, err)
		assert.Equal(t, fixture.templateID, member.RoleID)
	})

	t.Run("A member search by the template's ID finds a member on the copy", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		found, err := MembershipService.SearchMembers(t.Context(), organization.SearchMembersInput{
			ProductID: fixture.productID, OrganizationID: fixture.organizationID,
			Request: search.Request[organization.SearchMembersFilter, organization.SortFieldMember]{
				Filter:     &organization.SearchMembersFilter{RoleIDs: []string{fixture.templateID}},
				Pagination: search.Pagination{Limit: 10},
			},
		})

		require.NoError(t, err)
		require.Len(t, found.Items, 1)
		assert.Equal(t, fixture.userID, found.Items[0].ProductUserID)
	})

	t.Run("A template whose copy a member holds can't be deleted", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		err := ProductRoleService.DeleteProductRole(t.Context(), role.DeleteProductRoleInput{
			ProductID: fixture.productID, ID: fixture.templateID,
		})

		assertFaultCode(t, err, "ROLE_IN_USE")
	})

	t.Run("A template ID named in the Organization resolves to its copy", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		resolved, err := OrganizationRoleService.Resolve(
			t.Context(), fixture.productID, fixture.organizationID, fixture.templateID,
		)

		require.NoError(t, err)
		assert.Equal(t, fixture.copyID, resolved.RowID)
	})

	t.Run("A permission removed from the template leaves the copy", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)

		require.NoError(t, ProductRoleService.UnassignPermissionFromProductRole(t.Context(),
			role.UnassignPermissionFromProductRoleInput{
				ProductID: fixture.productID, ProductRoleID: fixture.templateID, PermissionName: fixture.permission,
			}))

		assert.Empty(t, rolePermissions(t, fixture.copyID))
	})

	t.Run("A permission given to the template reaches the copy", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)
		added := givenPermission(t, fixture.productID)

		require.NoError(t, ProductRoleService.AssignPermissionToProductRole(t.Context(),
			role.AssignPermissionToProductRoleInput{
				ProductID: fixture.productID, ProductRoleID: fixture.templateID, PermissionName: added,
			}))

		assert.ElementsMatch(t, []string{fixture.permission, added}, rolePermissions(t, fixture.copyID))
	})

	t.Run("A renamed template renames the copy", func(t *testing.T) {
		fixture := givenACopyHeldByAMember(t)
		name := "Renamed " + uuid.NewString()

		_, err := ProductRoleService.UpdateProductRole(t.Context(), role.UpdateProductRoleInput{
			ProductID: fixture.productID, ID: fixture.templateID, Name: &name,
		})

		require.NoError(t, err)
		var copyName string
		require.NoError(t, TestDB.QueryRowContext(t.Context(),
			`SELECT name FROM product_roles WHERE id = $1`, fixture.copyID).Scan(&copyName))
		assert.Equal(t, name, copyName)
	})
}

// The release before copies were made wrote memberships that hold the
// template itself and Organizations with no copies. Only the startup backfill
// moves them, and no route reaches it.
func TestCopyBackfill(t *testing.T) {
	t.Run("Every organization gets a copy with the template's permissions", func(t *testing.T) {
		ctx := t.Context()
		fixture := givenAMemberOnATemplate(t)

		require.NoError(
			t,
			orgroleservice.BackfillCopies(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		copied := requireCopy(t, fixture)
		assert.Equal(t, []string{fixture.permission}, rolePermissions(t, copied.RowID))
	})

	t.Run("A member on the template moves onto the copy", func(t *testing.T) {
		ctx := t.Context()
		fixture := givenAMemberOnATemplate(t)

		require.NoError(
			t,
			orgroleservice.BackfillCopies(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		var held string
		require.NoError(t, TestDB.QueryRowContext(ctx, `SELECT product_role_id FROM organization_memberships
			WHERE organization_id = $1 AND product_user_id = $2`, fixture.organizationID, fixture.userID).Scan(&held))
		assert.Equal(t, requireCopy(t, fixture).RowID, held)
	})

	t.Run("A copy left without its permissions gets them back", func(t *testing.T) {
		ctx := t.Context()
		fixture := givenACopyHeldByAMember(t)
		_, err := TestDB.ExecContext(ctx,
			`DELETE FROM product_role_resource_permissions WHERE product_role_id = $1`, fixture.copyID)
		require.NoError(t, err)

		require.NoError(
			t,
			orgroleservice.BackfillCopies(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		assert.Equal(t, []string{fixture.permission}, rolePermissions(t, fixture.copyID))
	})

	t.Run("Running the backfill again changes nothing", func(t *testing.T) {
		ctx := t.Context()
		fixture := givenAMemberOnATemplate(t)
		require.NoError(
			t,
			orgroleservice.BackfillCopies(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)
		first := requireCopy(t, fixture)

		require.NoError(
			t,
			orgroleservice.BackfillCopies(ctx, ProductRepository, OrganizationRoleRepo, Transactor, TestLogger),
		)

		var copies int
		require.NoError(t, TestDB.QueryRowContext(ctx, `SELECT count(*) FROM product_roles
			WHERE organization_id = $1 AND template_id = $2`, fixture.organizationID, fixture.templateID).Scan(&copies))
		assert.Equal(t, 1, copies)
		assert.Equal(t, first.RowID, requireCopy(t, fixture).RowID)
	})
}

// givenAMemberOnATemplate writes what the release before copies wrote: a
// template with one permission and an Organization whose member holds the
// template itself.
func givenAMemberOnATemplate(t *testing.T) copyFixture {
	t.Helper()
	ctx := t.Context()
	productID := GivenATenantAndProduct(t).Product.ID
	permission := givenPermission(t, productID)
	template := role.ProductRole{ProductID: productID, Name: "Legacy Admin " + uuid.NewString()}
	template.Key = strings.ToLower(template.Name)
	template.GenerateID()
	_, err := ProductRoleRepo.Create(ctx, template)
	require.NoError(t, err)
	givenRolePermission(t, productID, template.ID, permission)
	org := organization.Organization{ProductID: productID, Name: "Org " + uuid.NewString()}
	org.GenerateID()
	_, err = OrganizationRepo.Create(ctx, org)
	require.NoError(t, err)
	userID := givenProductUser(t, productID)
	_, err = TestDB.ExecContext(ctx, `INSERT INTO organization_memberships
		(organization_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`, org.ID, userID, template.ID)
	require.NoError(t, err)
	return copyFixture{
		productID: productID, organizationID: org.ID, templateID: template.ID, userID: userID, permission: permission,
	}
}

func requireCopy(t *testing.T, fixture copyFixture) organizationrole.Role {
	t.Helper()
	copied, err := OrganizationRoleRepo.Find(t.Context(), fixture.productID, fixture.organizationID, fixture.templateID)
	require.NoError(t, err)
	require.True(t, copied.IsPresent())
	require.NotNil(t, copied.Value().TemplateID)
	return copied.Value()
}

type copyFixture struct {
	productID      string
	organizationID string
	templateID     string
	copyID         string
	userID         string
	permission     string
}

// givenACopyHeldByAMember writes a template with one permission, its copy in
// an Organization with the same permission, and a member who holds the copy.
func givenACopyHeldByAMember(t *testing.T) copyFixture {
	t.Helper()
	ctx := t.Context()
	productID := GivenATenantAndProduct(t).Product.ID
	permission := givenPermission(t, productID)
	template := role.ProductRole{ProductID: productID, Name: "Editor " + uuid.NewString()}
	template.Key = strings.ToLower(template.Name)
	template.GenerateID()
	_, err := ProductRoleRepo.Create(ctx, template)
	require.NoError(t, err)
	givenRolePermission(t, productID, template.ID, permission)
	org := organization.Organization{ProductID: productID, Name: "Org " + uuid.NewString()}
	org.GenerateID()
	_, err = OrganizationRepo.Create(ctx, org)
	require.NoError(t, err)
	copyID := "product_role_" + uuid.NewString()
	_, err = TestDB.ExecContext(ctx, `INSERT INTO product_roles
		(id, product_id, organization_id, template_id, key, name) VALUES ($1, $2, $3, $4, $5, $6)`,
		copyID, productID, org.ID, template.ID, template.Key, template.Name)
	require.NoError(t, err)
	givenRolePermission(t, productID, copyID, permission)
	userID := givenProductUser(t, productID)
	_, err = TestDB.ExecContext(ctx, `INSERT INTO organization_memberships
		(organization_id, product_user_id, product_role_id) VALUES ($1, $2, $3)`, org.ID, userID, copyID)
	require.NoError(t, err)
	return copyFixture{
		productID: productID, organizationID: org.ID, templateID: template.ID,
		copyID: copyID, userID: userID, permission: permission,
	}
}

func givenUnkeyedTemplate(t *testing.T, productID, name string) role.ProductRole {
	t.Helper()
	template := role.ProductRole{ProductID: productID, Name: name}
	template.GenerateID()
	_, err := ProductRoleRepo.Create(t.Context(), template)
	require.NoError(t, err)
	_, err = TestDB.ExecContext(t.Context(), `UPDATE product_roles SET key = NULL WHERE id = $1`, template.ID)
	require.NoError(t, err)
	return template
}

func givenPermission(t *testing.T, productID string) string {
	t.Helper()
	name := "doc:" + uuid.NewString()
	_, err := TestDB.ExecContext(t.Context(),
		`INSERT INTO product_resource_permissions (product_id, name) VALUES ($1, $2)`, productID, name)
	require.NoError(t, err)
	return name
}

func givenRolePermission(t *testing.T, productID, roleID, permission string) {
	t.Helper()
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO product_role_resource_permissions
		(id, product_id, product_role_id, permission_name) VALUES ($1, $2, $3, $4)`,
		"product_role_resource_permission_"+uuid.NewString(), productID, roleID, permission)
	require.NoError(t, err)
}

func givenProductUser(t *testing.T, productID string) string {
	t.Helper()
	userID := "pusr_" + uuid.NewString()
	_, err := TestDB.ExecContext(t.Context(), `INSERT INTO product_users (id, product_id, email, status)
		VALUES ($1, $2, $3, 'ACTIVE')`, userID, productID, uuid.NewString()+"@example.com")
	require.NoError(t, err)
	return userID
}

func templateKey(t *testing.T, templateID string) string {
	t.Helper()
	var key string
	require.NoError(t, TestDB.QueryRowContext(t.Context(),
		`SELECT key FROM product_roles WHERE id = $1`, templateID).Scan(&key))
	return key
}

func rolePermissions(t *testing.T, roleID string) []string {
	t.Helper()
	rows, err := TestDB.QueryContext(t.Context(),
		`SELECT permission_name FROM product_role_resource_permissions WHERE product_role_id = $1`, roleID)
	require.NoError(t, err)
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())
	return names
}

func assertFaultCode(t *testing.T, err error, code string) {
	t.Helper()
	failure, isFault := fault.As(err)
	require.True(t, isFault, "expected a fault, got %v", err)
	require.NotEmpty(t, failure.Details)
	assert.Equal(t, code, failure.Details[0].Code)
}

func TestEnsureCopiesBeyondOneBatch(t *testing.T) {
	t.Run("An organization gets a copy of every template when there are more than one batch", func(t *testing.T) {
		ctx := t.Context()
		productID := GivenATenantAndProduct(t).Product.ID
		const templates = 501
		_, err := TestDB.ExecContext(ctx, `INSERT INTO product_roles (id, product_id, key, name)
			SELECT 'product_role_' || gen_random_uuid(), $1, 'k' || n, 'Role ' || n
			FROM generate_series(1, $2) AS n`, productID, templates)
		require.NoError(t, err)
		org := organization.Organization{ProductID: productID, Name: "Org " + uuid.NewString()}
		org.GenerateID()
		_, err = OrganizationRepo.Create(ctx, org)
		require.NoError(t, err)

		require.NoError(t, Transactor.InTx(ctx, func(txCtx context.Context) error {
			return OrganizationRoleService.EnsureCopies(txCtx, productID, org.ID)
		}))

		var copies int
		require.NoError(t, TestDB.QueryRowContext(ctx,
			`SELECT count(*) FROM product_roles WHERE organization_id = $1`, org.ID).Scan(&copies))
		assert.Equal(t, templates, copies)
	})
}
